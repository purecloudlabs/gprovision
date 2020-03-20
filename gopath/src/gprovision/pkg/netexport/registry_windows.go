// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package netexport

import (
	"fmt"
	"gprovision/pkg/log"
	inet "gprovision/pkg/net"
	"io"
	"net"
	"strings"

	reg "golang.org/x/sys/windows/registry"
)

//https://godoc.org/golang.org/x/sys/windows/registry#OpenKey

const (
	HALInterfaces = `SYSTEM\CurrentControlSet\Control\Network\{4D36E972-E325-11CE-BFC1-08002BE10318}`
	v4Interfaces  = `SYSTEM\CurrentControlSet\Services\Tcpip\Parameters\Interfaces`
	v6Interfaces  = `SYSTEM\CurrentControlSet\Services\TCPIP6\Parameters\Interfaces`
	luidData      = `SYSTEM\CurrentControlSet\Control\Nsi\{eb004a11-9b1a-11d4-9123-0050047759bc}\10`
	v6Unicast     = `SYSTEM\CurrentControlSet\Control\Nsi\{eb004a01-9b1a-11d4-9123-0050047759bc}\10`
	//multicast is same as v6Unicast, except ...\8 - not useful?
)

//maps from FriendlyName to a NetCfgInstanceId guid
var guidMap map[string]string

//maps from guid to net_luid (need net_luid for v6 IPs)
var luidMap map[string]string

func init() {
	guidMap = make(map[string]string)
	luidMap = make(map[string]string)
}

//given interface index, return interface guid
func guidFromName(name string) string {
	if len(guidMap) == 0 {
		populateGuidMap()
	}
	return guidMap[name]
}

func populateGuidMap() {
	//HKEY_LOCAL_MACHINE\SYSTEM\CurrentControlSet\Control\Network\{4D36E972-E325-11CE-BFC1-08002BE10318}\{AB97FBFA-A41A-49D4-9D75-56B8EA98BD96}\Connection

	k, err := reg.OpenKey(reg.LOCAL_MACHINE, HALInterfaces, reg.READ)
	if err != nil {
		panic(fmt.Sprintf("populateGuidMap() failed to open reg key: %s\n", err))
	}
	defer k.Close()
	subkeys, err := k.ReadSubKeyNames(100)
	if err == io.EOF {
		err = nil
	}
	if err != nil {
		panic(fmt.Sprintf("populateGuidMap() failed to read subkeys: %s\nkey:%#v\n", err, k))
	}
	if len(subkeys) == 100 {
		//shouldn't ever get here
		panic("populateGuidMap(): too many subkeys")
	}
	for _, guid := range subkeys {
		if guid == "Descriptions" {
			continue
		}
		if guid[0] != '{' {
			log.Logln("not a guid:", guid)
			continue
		}
		sk, err := reg.OpenKey(reg.LOCAL_MACHINE, HALInterfaces+`\`+guid+`\Connection`, reg.READ)
		if err != nil {
			log.Logf("populateGuidMap() failed to open subkey %s: %s\n", guid, err)
			continue
		}
		defer sk.Close()
		name, _, err := sk.GetStringValue("Name")
		if err != nil || name == "" {
			log.Logf("populateGuidMap() failed to read Name of %s: %s\n", guid, err)
			continue
		}
		guidMap[name] = guid
	}
}

//given interface guid, return interface's NET_LUID
func luidFromGuid(guid string) string {
	if len(luidMap) == 0 {
		populateLuidMap()
	}
	luid, ok := luidMap[guid]
	if !ok {
		log.Logf("luidFromGuid(): failed to read luid for guid %s\n", guid)
	}
	return luid
}

func populateLuidMap() {
	//[HKEY_LOCAL_MACHINE\SYSTEM\CurrentControlSet\Control\Nsi\{eb004a11-9b1a-11d4-9123-0050047759bc}\10]
	k, err := reg.OpenKey(reg.LOCAL_MACHINE, luidData, reg.READ)
	if err != nil {
		panic(fmt.Sprintf("populateLuidMap() failed to open reg key: %s\n", err))
	}
	defer k.Close()
	luids, err := k.ReadValueNames(100)
	if err == io.EOF {
		err = nil
	}
	if err != nil {
		panic(fmt.Sprintf("populateLuidMap() failed to read values: %s\nkey:%#v\n", err, k))
	}
	if len(luids) == 100 {
		//shouldn't ever get here
		panic("populateLuidMap(): too many values")
	}
	for _, luid := range luids {
		//0000000000008300
		if len(luid) != 16 {
			log.Logf("skipping %s, len=%d\n", luid, len(luid))
			continue
		}
		val, _, err := k.GetBinaryValue(luid)
		if err != nil {
			log.Logf("populateLuidMap() failed to read value of %s: %s\n", luid, err)
			continue
		}
		if len(val) < 0x470 {
			log.Logf("populateLuidMap(): skipping %s, struct is unexpectedly short (0x%x < 0x470)\n", luid, len(val))
			continue
		}
		//https://blogs.msdn.microsoft.com/openspecification/2013/10/08/guids-and-endianness-endi-an-ne-ssinguid-or-idne-na-en-ssinguid/
		binGuid := val[0x410:0x420]
		guid := guidStrFromRegBin(binGuid)
		if len(guid) != 38 {
			panic(fmt.Sprintf("bad guid len (%d): %s", len(guid), guid))
		}
		luidMap[guid] = luid
	}

}

//comma-delimited list of persistent IP addresses for the interface
func PersistentIPs(name string) (ips []inet.IPNet, dhcp dhcp46) {
	guid := guidFromName(name)
	v4 := ifIPv4Addrs(guid)
	v6 := ifIPv6Addrs(guid)
	dhcp.v4 = (len(v4) == 0)
	dhcp.v6 = (len(v6) == 0)
	ips = append(v4, v6...)
	if len(ips) == 0 {
		log.Logln("no IPs for", name, guid)
	} else {
		log.Logln("IPs for", name, guid)
	}
	for _, i := range ips {
		log.Logf("    %s\n", i.String())
	}
	return
}

//get ipv6 addresses for a given interface; unfortunately much different than ipv4
func ifIPv6Addrs(guid string) (ips []inet.IPNet) {
	//https://stackoverflow.com/questions/8155700/how-to-add-persistent-ipv6-address-in-vista-windows7
	k, err := reg.OpenKey(reg.LOCAL_MACHINE, v6Unicast, reg.READ)
	if err != nil {
		log.Logf("ifIPv6Addrs() failed to read ipv6 unicast key: %s\n", err)
		return
	}
	names, err := k.ReadValueNames(100)
	if err == io.EOF {
		err = nil
	}
	if err != nil {
		log.Logf("ifIPv6Addrs() failed to read values for ipv6 unicast: %s\n", err)
		return
	}
	luid := luidFromGuid(guid)
	lluid := len(luid)
	if lluid == 0 {
		log.Logf("ifIPv6Addrs(): bad luid. guid=%s\n", guid)
		return
	}
	for _, name := range names {
		if strings.HasPrefix(name, luid) {
			rawip := name[lluid:]
			ip := xlateIPv6(rawip)
			if ip == nil {
				continue
			}
			data, _, err := k.GetBinaryValue(name)
			if err != nil {
				log.Logf("ifIPv6Addrs() failed to read key data for %s: %s\n", name, err)
				continue
			}
			v6data := parseV6Data(data)
			ipn := net.IPNet{
				IP:   ip,
				Mask: net.CIDRMask(int(v6data.prefixLen), net.IPv6len*8),
			}
			ips = append(ips, inet.IPNet{ipn})
		}
	}
	return ips
}

//inject colons into IP, trim 0's - returning canonical format
func xlateIPv6(rawip string) net.IP {
	ip := ""
	rlen := len(rawip)
	for i, c := range rawip {
		ip += string(c)
		if (i+1)%4 == 0 && i+1 < rlen {
			ip += ":"
		}
	}
	pip := net.ParseIP(ip)
	if pip == nil {
		log.Logf("cannot parse %s (%s) as ipv6\n", rawip, ip)
	}
	return pip
}

type ipv6data struct {
	validLife, preferLife, pfxOrigin, sfxOrigin uint32
	prefixLen                                   uint8
}

func parseV6Data(data []byte) ipv6data {
	/* https://stackoverflow.com/questions/8155700/how-to-add-persistent-ipv6-address-in-vista-windows7
	   typedef struct _UNKNOWN {
	     ULONG            ValidLifetime;
	     ULONG            PreferredLifetime;
	     NL_PREFIX_ORIGIN PrefixOrigin;
	     NL_SUFFIX_ORIGIN SuffixOrigin;
	     UINT8            OnLinkPrefixLength; //16
	     BOOLEAN          SkipAsSource;
	     UCHAR            Unknown[28];
	   } UNKNOWN;

	*/
	v6d := ipv6data{
		// validLife:  le.Uint32(data[:4])
		// preferLife: le.Uint32(data[4:8])
		// pfxOrigin:  le.Uint32(data[8:12])
		// sfxOrigin:  le.Uint32(data[12:16])
		prefixLen: data[16],
	}
	return v6d
}

func ifIPv4Addrs(guid string) (ips []inet.IPNet) {
	k, err := reg.OpenKey(reg.LOCAL_MACHINE, v4Interfaces+`\`+guid, reg.READ)
	if err != nil {
		log.Logf("ifIPv4Addrs() failed to open key %s: %s\n", guid, err)
		return
	}
	defer k.Close()
	addrs, _, erra := k.GetStringsValue("IPAddress")
	subnets, _, errs := k.GetStringsValue("SubnetMask")
	if erra == reg.ErrNotExist && errs == reg.ErrNotExist {
		return nil
	}
	if erra == reg.ErrNotExist {
		log.Logf("no ipv4s found for %s\n", guid)
		return nil
	}
	if erra != nil {
		log.Logf("error reading ipv4 for %s: %s\n", guid, erra)
		return nil
	}
	if errs == reg.ErrNotExist {
		log.Logf("no subnets found for %s\n", guid)
		return nil
	}
	if errs != nil {
		log.Logf("error reading ipv4 subnet for %s: %s\n", guid, errs)
		return nil
	}
	if len(addrs) != len(subnets) {
		log.Logf("unequal number of ipv4 addresses and subnets for for %s\n", guid)
		return nil
	}
	for i, a := range addrs {
		ip := net.ParseIP(a)
		mask := maskFromString(subnets[i])
		ips = append(ips, inet.IPNet{net.IPNet{
			IP:   ip,
			Mask: mask,
		}})
	}
	return ips
}

//retrieve comma-delimited list of dns servers
func PersistentDNS(name string) string {
	guid := guidFromName(name)
	v4 := ifDNSbyType(guid, v4Interfaces)
	v6 := ifDNSbyType(guid, v6Interfaces)
	log.Logln("dns for", name, guid, v4, v6)
	if len(v4) > 0 && len(v6) > 0 {
		return v4 + "," + v6
	}
	if len(v4) > 0 {
		return v4
	}
	return v6
}

//returns comma-delimited list of DNS servers
func ifDNSbyType(guid, ipType string) string {
	//[HKEY_LOCAL_MACHINE\SYSTEM\CurrentControlSet\Services\Tcpip\Parameters\Interfaces\{1D1BD1A2-0FD9-41E9-BBB5-A98BAC570B2A}]
	k, err := reg.OpenKey(reg.LOCAL_MACHINE, ipType+`\`+guid, reg.READ)
	if err != nil {
		log.Logf("ifDNSbyType() failed to open key %s: %s\n", guid, err)
		return ""
	}
	defer k.Close()
	ns, _, err := k.GetStringValue("NameServer")
	if err == reg.ErrNotExist {
		return ""
	}
	if err != nil {
		log.Logf("ifDNSbyType() failed to read value for %s: %s\n", guid, err)
	}
	//fix delimiters, just in case https://www.welivesecurity.com/2016/06/02/crouching-tiger-hidden-dns/
	ns = strings.Replace(ns, ";", ",", -1)
	return strings.Replace(ns, " ", ",", -1)
}
