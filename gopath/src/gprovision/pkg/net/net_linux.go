// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

//Package net implements some network-related functions for recovery.
//These include enabling interfaces and acquiring addresses via DHCP,
//and downloading files.
package net

import (
	"gprovision/pkg/hw/ioctl"
	"gprovision/pkg/hw/nic"
	"gprovision/pkg/log"
	"net"
	"os/exec"
	"time"
)

//Enable any network interface, even DIAG if not hidden
func EnableNetworkingAny() bool {
	ifaces := nic.List()
	log.Logf("trying to enable networking on any of %v", ifaces)
	return enableNetworkingIfaceList(nil, ifaces)
}

// Enable any network interface except DIAG (only use if DIAG port(s) are not
// already hidden - if they are, usable ports will be skipped)
func EnableNetworkingSkipDIAG(diags []int, allowedPrefixes [][]byte) (success bool) {
	ifaces := nic.SortedList(allowedPrefixes)
	log.Logf("Found %d interfaces with allowed prefixes", len(ifaces))
	if len(ifaces) == 0 {
		return false
	}
	return enableNetworkingIfaceList(diags, ifaces)
}

// bring interfaces up and try to acquire addr via dhcp, wait for any one to
// gain ipv4
func enableNetworkingIfaceList(diags []int, ifaces []nic.Nic) (success bool) {
ifloop:
	for idx, iface := range ifaces {
		for _, d := range diags {
			if d == idx {
				//this is a diag port, move on to the next
				log.Logf("skipping diag port %s", iface.Mac().String())
				continue ifloop
			}
		}
		name := iface.String()
		netif, err := net.InterfaceByName(name)
		if err != nil {
			log.Logf("interface %s: error %s", name, err)
			continue
		}
		if ioctl.NicIsUp(name) && HasIpv4(netif) {
			log.Logf("skipping interface %s, already has an address", name)
			continue
		}

		log.Logf("enabling network interface %s", name)
		//TODO set hostname?
		go EnableNic(name)
	}
	return WaitForIpv4(time.Minute*5, ifaces)
}

//set state up, get addr via dhcp
func EnableNic(nic string) (success bool) {
	up := true
	err := ioctl.SetNicState(nic, up)
	if err != nil {
		log.Logf("error enabling %s: %s\n", nic, err)
		return false
	}
	time.Sleep(time.Second)
	script := "/usr/share/udhcpc/default.script"
	dhcp := exec.Command("udhcpc",
		"-i", nic, //interface
		"-t", "3", //retries
		"-T", "3", //timeout
		"-A", "10", //retry delay
		"-s", script, //script that actually configures the interface
		"-q", //quit on obtain
		"-b") //background if no lease

	out, e := dhcp.CombinedOutput()
	if e != nil {
		log.Logf("DHCP error %s for if %s. Output:\n%s\n", e, nic, out)
		return false
	}
	return true
}
