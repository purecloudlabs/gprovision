// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package net

import (
	"bytes"
	"fmt"
	"net"
	"strings"

	"github.com/purecloudlabs/gprovision/pkg/log"
)

// IPNet is a wrapper around net.IPNet allowing us to extend the functionality.
type IPNet struct {
	net.IPNet
}

func (ip *IPNet) MarshalJSON() (data []byte, err error) {
	data = []byte(fmt.Sprintf(`"%s"`, ip.String()))
	return
}

func (ip *IPNet) UnmarshalJSON(data []byte) error {
	i := strings.Trim(string(data), `"`)
	ipn, err := IPNetFromCIDR(i)
	if err == nil {
		ip.IPNet = ipn.IPNet
	}
	return err
}
func (l IPNet) Equal(r IPNet) bool {
	if !bytes.Equal(l.Mask, r.Mask) {
		return false
	}
	return l.IPNet.IP.Equal(r.IPNet.IP)
}

// IPNetFromCIDR converts an address string in CIDR notation into an IPNet.
// If you use net.ParseCIDR directly, the returned IPNet contains the subnet
// and mask, not the ip and mask.
func IPNetFromCIDR(cidr string) (IPNet, error) {
	if !strings.Contains(cidr, "/") {
		if strings.Contains(cidr, ":") {
			//ipv6
			cidr += "/128"
		} else {
			cidr += "/32"
		}
	}
	ip := IPNet{}
	addr, ipnet, err := net.ParseCIDR(cidr)
	if err == nil {
		ip.IP = addr
		ip.Mask = ipnet.Mask //ipnet.IP is the subnet
	}
	return ip, err
}

func IPNetFromAddr(a net.Addr) (IPNet, error) { return IPNetFromCIDR(a.String()) }

func IPNetsFromIface(iface string) ([]IPNet, error) {
	wanif, err := net.InterfaceByName(iface)
	if err != nil {
		return nil, err
	}
	addrs, err := wanif.Addrs()
	if err != nil {
		return nil, err
	}
	var ipn []IPNet
	for _, a := range addrs {
		ip, err := IPNetFromAddr(a)
		if err != nil {
			return nil, err
		}
		ipn = append(ipn, ip)
	}
	return ipn, nil
}

// HasIpv4 returns true if the given interface has an ipv4 address.
func HasIpv4(netif *net.Interface) bool {
	addrs, _ := netif.Addrs()
	for _, addr := range addrs {
		ip, _, err := net.ParseCIDR(addr.String())
		if err != nil {
			log.Logf("error %s parsing interface %s address %s", err, netif.Name, addr.String())
			continue
		}
		if ip.To4() != nil {
			return true
		}
	}
	return false
}
