// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package netexport

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"net"
	"os/exec"
	"strconv"
	"strings"

	"github.com/purecloudlabs/gprovision/pkg/log"
	inet "github.com/purecloudlabs/gprovision/pkg/net"
)

const (
	powershellExe = `C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`
)

type dhcp46 struct {
	v4 bool
	v6 bool
}

//get ipv4 and ipv6 addresses for interfaces
//return comma-separated list of interfaces we care about - these are for use with powershell
func (ifmap IfMap) GetAddrs() (err error) {
	//seems microsoft uses this for several different virtual interfaces
	MSFT, err := net.ParseMAC("00:00:00:00:00:00:00:e0")
	if err != nil {
		panic(fmt.Sprintf("failed to parse mac: %s", err))
	}
	ifaces, err := net.Interfaces()
	if err != nil {
		log.Logf("interface read error: %s\n", err)
	}
	for _, i := range ifaces {
		if (i.Flags & net.FlagLoopback) != 0 {
			continue
		}
		if bytes.Equal(i.HardwareAddr, MSFT) {
			log.Logln("skipping MS interface", i.Name)
			continue
		}
		if strings.HasPrefix(i.Name, "isatap.") || strings.HasPrefix(strings.ToLower(i.Name), "teredo") {
			continue
		}
		_, exists := ifmap[i.Index]
		if exists {
			log.Logf("interface %s already exists", i.Name)
			continue
		}
		addrs, dhcp := PersistentIPs(i.Name)
		ifmap[i.Index] = &WinNic{
			Mac:          StringyMac{i.HardwareAddr},
			WinIndex:     i.Index,
			FriendlyName: i.Name,
			IPs:          addrs,
			DHCP4:        dhcp.v4, //these are turned off at merge if this interface has VLAN children
			DHCP6:        dhcp.v6,
		}
	}
	ifmap.getDns()
	err = ifmap.getPersistentRoutes()
	if err != nil {
		log.Logf("%s\r\n", err)
	}
	return
}

func runPsCmd(cmd string) (out []byte, err error) {
	formatted := fmt.Sprintf("%s | Ft -autosize -hidetableheaders | out-string -width 4096", cmd)
	ps := exec.Command(powershellExe, "-Command", formatted)
	out, err = ps.CombinedOutput()
	return
}

func (ifmap IfMap) getDns() {
	for _, iface := range ifmap {
		servers := PersistentDNS(iface.FriendlyName)
		if len(servers) == 0 {
			continue
		}
		for _, srv := range strings.Split(servers, ",") {
			if len(srv) == 0 {
				continue
			}
			iface.NameServers = append(iface.NameServers, net.ParseIP(srv))
		}
	}
}
func (ifmap IfMap) getPersistentRoutes() error {
	routeout, err := runPsCmd("get-netroute -policystore persistentstore|select ifIndex,DestinationAddress,IsStatic,RouteMetric,TypeOfRoute,DestinationPrefix,NextHop|convertto-csv -notypeinformation|select -skip 1")
	if err != nil {
		return err
	}
	log.Logf("Routes\n======\n%s\n======\n", string(routeout))
	return ifmap.parseRoutes(routeout)
}

/*
ifIndex DestinationPrefix   NextHop      RouteMetric   PolicyStore
------- -----------------   -------      -----------   -----------
26      0.0.0.0/0           10.155.8.1           256   Persiste...
13      0.0.0.0/0           10.155.8.1           256   Persiste...
14      ::/0                27::1                256   Persiste...
*/

/*get-netroute -policystore persistentstore|convertto-csv -notypeinformation|select -skip 1
"ifIndex","Publish","Store","AddressFamily","Caption","Description","ElementName","InstanceID","AdminDistance",
"DestinationAddress","IsStatic","RouteMetric","TypeOfRoute","DestinationPrefix","InterfaceAlias","InterfaceIndex",
"NextHop","PreferredLifetime","ValidLifetime","PSComputerName"
"26","No","PersistentStore","IPv4",,,,":8:8:8:9:55<@55;:8;??8B8;55:",,
,,"256","3","0.0.0.0/0","Port 5 - VLAN 88","26",
"10.155.8.1","10675199.02:48:05.4775807","10675199.02:48:05.4775807",

http://wutils.com/wmi/root/standardcimv2/msft_netroute/

TypeOfRoute 2/3/4 -> ['Administrator Defined Route', 'Computed Route', 'Actual Route']

fields we don't care about: Publish,Store,AddressFamily,Caption,Description,ElementName,InstanceID,AdminDistance,InterfaceAlias,InterfaceIndex,PreferredLifetime,ValidLifetime,PSComputerName
get-netroute -policystore persistentstore|select ifIndex,DestinationAddress,IsStatic,RouteMetric,TypeOfRoute,DestinationPrefix,NextHop|convertto-csv -notypeinformation|select -skip 1
                                                    0          1              2          3           4            5               6
*/
var defaultRoute4, defaultRoute6 *net.IPNet
var single4, single6 net.IPMask

func init() {
	var err error
	_, defaultRoute4, err = net.ParseCIDR("0.0.0.0/0")
	if err != nil {
		panic(err)
	}
	_, defaultRoute6, err = net.ParseCIDR("::/0")
	if err != nil {
		panic(err)
	}
	single4 = net.CIDRMask(32, 32)
	single6 = net.CIDRMask(128, 128)
}

func (ifmap IfMap) parseRoutes(routeout []byte) (err error) {
	data := bytes.NewReader(routeout)
	r := csv.NewReader(data)
	for {
		var record []string
		record, err = r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			err = fmt.Errorf("error %s reading a record\n", err)
			return
		}
		if len(record) != 7 {
			err = fmt.Errorf("record %v has bad len %d\n", record, len(record))
			return
		}
		var idx, metric int64
		idx, err = strconv.ParseInt(record[0], 10, 64)
		if err != nil {
			return fmt.Errorf("parse index: %s", err)
		}
		destAddr := net.ParseIP(strings.TrimSpace(record[1]))
		// static := (record[2] != "") //empty -> dynamic / false
		metric, err = strconv.ParseInt(strings.TrimSpace(record[3]), 10, 64)
		if err != nil {
			err = fmt.Errorf("err %s parsing record %v", err, record)
			return
		}
		// rtype := strings.TrimSpace(record[4]) //ignore route type? or ignore routes except when this is 2?
		var destPfx *net.IPNet
		pfx := strings.TrimSpace(record[5])
		if pfx != "" {
			_, destPfx, err = net.ParseCIDR(pfx)
			if err != nil {
				return
			}
		}
		nextHop := net.ParseIP(strings.TrimSpace(record[6]))
		iface, ok := ifmap[int(idx)]
		if !ok {
			err = fmt.Errorf("interface index %d missing for record %v\n", idx, record)
			return
		}
		//not sure why windows provides DestinationAddress _and_ DestinationPrefix... I assume they are mutually exclusive
		if (destAddr == nil) == (destPfx == nil) {
			err = fmt.Errorf("record %v: require exactly one of dest addr, dest prefix\n", record)
			return
		}
		var route Route
		if destPfx != nil {
			route.Destination = inet.IPNet{*destPfx}
		} else {
			route.Destination.IP = destAddr
			if destAddr.To4() != nil {
				route.Destination.Mask = single4
			} else {
				route.Destination.Mask = single6
			}
		}
		route.Gateway = nextHop
		route.Metric = int(metric)
		iface.Routes = append(iface.Routes, route)

	}
	return nil
}
