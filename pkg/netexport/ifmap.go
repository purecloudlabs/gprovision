// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package netexport

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"strings"

	"github.com/purecloudlabs/gprovision/pkg/log"
	inet "github.com/purecloudlabs/gprovision/pkg/net"
)

type WinNic struct {
	FriendlyName    string //e.g. "Port 1 (WAN)", "Port 2 - VLAN 58"
	Mac             StringyMac
	WinName         string //"Intel(R) I350 Gigabit Network Connection #2 - VLAN : PureCloud Vlan=58"
	WinIndex        int
	DHCP4           bool
	DHCP6           bool
	IPs             []inet.IPNet
	Routes          RouteList
	NameServers     []net.IP
	HasVLANChildren bool //only true if this is a "real" interface that has vlans configured
	IsVLAN          bool
	VLAN            uint64
}

type IfMap map[int]*WinNic

func NewIfMap() IfMap { return make(map[int]*WinNic) }

func (i IfMap) ToJson() (encoded []byte, err error) {
	encoded, err = json.Marshal(i)
	return
}

func (nic WinNic) MarshalJSON() ([]byte, error) {
	//first := true
	s := "{"
	s += fmt.Sprintf(`"FriendlyName":"%s",`, nic.FriendlyName)
	s += fmt.Sprintf(`"Mac":"%s",`, nic.Mac.String())
	wn, err := json.Marshal(nic.WinName)
	if err != nil {
		log.Logf("WinName: %s\n", err)
	} else {
		s += fmt.Sprintf(`"WinName": %s,`, wn)
	}
	s += fmt.Sprintf(`"DHCP4": %t,`, nic.DHCP4)
	s += fmt.Sprintf(`"DHCP6": %t,`, nic.DHCP6)
	if len(nic.IPs) > 0 {
		ips, err := json.Marshal(nic.IPs)
		if err != nil {
			log.Logf("IPs: %s\n", err)
			return nil, err
		}
		s += fmt.Sprintf(`"IPs":%s,`, ips)
	}
	if len(nic.Routes) > 0 {
		routes, err := json.Marshal(nic.Routes)
		if err != nil {
			log.Logf("Routes: %s\n", err)
			return nil, err
		}
		s += fmt.Sprintf(`"Routes":%s,`, routes)
	}
	if len(nic.NameServers) > 0 {
		ns, err := json.Marshal(nic.NameServers)
		if err != nil {
			log.Logf("NSs: %s\n", err)
			return nil, err
		}
		s += fmt.Sprintf(`"NameServers":%s,`, ns)
	}
	if nic.HasVLANChildren {
		s += fmt.Sprintf(`"HasVLANChildren":%t,`, nic.HasVLANChildren)
	}
	s += fmt.Sprintf(`"IsVLAN":%t`, nic.IsVLAN)
	if nic.IsVLAN {
		s += fmt.Sprintf(`,"VLAN":%d`, nic.VLAN)
	}
	s += "}"
	return []byte(s), nil
}

type RouteList []Route

type Route struct {
	Destination inet.IPNet
	Gateway     net.IP
	Metric      int
}

func (r Route) MarshalJSON() ([]byte, error) {
	if skipRoute(r.Destination) {
		log.Logf("skipping route to %s\n", r.Destination.String())
		return []byte{}, nil
	}
	s := "{"
	s += fmt.Sprintf(`"Destination":"%s",`, r.Destination.String())
	if r.Gateway != nil {
		s += fmt.Sprintf(`"Gateway":"%s",`, r.Gateway.String())
	}
	s += fmt.Sprintf(`"Metric":%d`, r.Metric)
	s += "}"
	return []byte(s), nil
}

func skipRoute(nw inet.IPNet) bool {
	return nw.IP.IsLinkLocalUnicast() || nw.IP.IsLinkLocalMulticast() || nw.IP.IsMulticast()
}

func (ifaces IfMap) Merge(intelData IntelMap) {
	vlanParent := 9990
	for _, i := range intelData {
		found := false
		for _, j := range ifaces {
			if i.FriendlyName != j.FriendlyName {
				continue
			}
			found = true
			j.WinName = i.WinName
			j.VLAN = i.VLAN
			j.HasVLANChildren = i.HasVLANChildren
			j.IsVLAN = i.IsVLAN
			if j.HasVLANChildren {
				j.DHCP4 = false
				j.DHCP6 = false
			}
			break
		}
		if !found {
			if i.HasVLANChildren {
				for {
					//find an unused index, since we don't know the number windows would use
					_, present := ifaces[vlanParent]
					if present {
						vlanParent++
						continue
					}
					ifaces[vlanParent] = i
					break
				}
			}
		}
	}
}

func (ifaces IfMap) VlanChildren(iface *WinNic) (vlans []uint64) {
	if iface.HasVLANChildren {
		//look up vlans
		for _, child := range ifaces {
			if child.IsVLAN && bytes.Equal(child.Mac.HardwareAddr, iface.Mac.HardwareAddr) {
				vlans = append(vlans, child.VLAN)
			}
		}
	}
	return
}

//Create a unique interface name for a vlan. Avoids `enoN.vlan` as at the point this runs we have
// no idea what linux will choose for the parent's interface name. Output is like `port1.123`.
func (nic WinNic) VlanIfName(vlan uint64) (ifname string) {
	var base string
	idx := strings.Index(nic.FriendlyName, " - VLAN ")

	if nic.HasVLANChildren == nic.IsVLAN {
		panic("nic.HasVLANChildren == nic.IsVLAN")
	}
	if nic.HasVLANChildren {
		if idx != -1 {
			panic("VLAN in parent's name")
		}
		base = nic.FriendlyName
	} else {
		if idx == -1 {
			panic("child name missing VLAN")
		}
		base = nic.FriendlyName[:idx]
	}
	name := fmt.Sprintf("%s.%d", base, vlan)
	return strings.Replace(strings.ToLower(name), " ", "", -1)
}
