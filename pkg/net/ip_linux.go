// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package net

import (
	"net"
	"time"

	"github.com/purecloudlabs/gprovision/pkg/hw/nic"
	"github.com/purecloudlabs/gprovision/pkg/log"

	"github.com/vishvananda/netlink"
)

// Assign the given IP to the given device. Call multiple times to assign
// multiple addresses.
func AssignIP(device string, addr net.IPNet) {
	dev, err := netlink.LinkByName(device)
	if err != nil {
		log.Logf("addr add failed: %s", err)
	}
	err = netlink.AddrAdd(dev, &netlink.Addr{IPNet: &addr})
	if err != nil {
		log.Logf("addr add failed: %s", err)
	}
}

// WaitForIpv4 waits until at least one of the given interfaces to gain an
// ipv4 address, or until the wait time has expired.
func WaitForIpv4(wait time.Duration, ifaces []nic.Nic) (success bool) {
	stopTime := time.Now().Add(wait)
	for time.Now().Before(stopTime) {
		time.Sleep(time.Second)
		for _, iface := range ifaces {
			name := iface.String()
			netif, err := net.InterfaceByName(name)
			if err != nil {
				log.Logf("interface %s: err %s\n", name, err)
				continue
			}
			if HasIpv4(netif) {
				return true
			}
		}
	}
	return false
}
