// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

//Package nic allows enabling/disabling NICs, configuring XPS, RFS, and RSS, etc.
package nic

import (
	"io/ioutil"
	"net"
	"os"
	fp "path/filepath"
	"strings"

	"github.com/purecloudlabs/gprovision/pkg/log"
)

const (
	sysClassNet = "/sys/class/net"
)

type Nic struct {
	device string
	mac    net.HardwareAddr
	irqs   []uint64
}
type NicList []Nic

//used to create a Nic for testing
func TestNic(device, mac string, irqs []uint64) (n Nic, err error) {
	n.device = device
	n.irqs = irqs
	n.mac, err = net.ParseMAC(mac)
	return
}

func (n Nic) String() string        { return n.device }
func (n Nic) Name() string          { return n.device }
func (n Nic) Mac() net.HardwareAddr { return n.mac }

//return array of names of nics
func List() (nics NicList) {
	contents, err := ioutil.ReadDir(sysClassNet)
	if err != nil {
		log.Logf("err reading dir %s: %s\n", sysClassNet, err)
		return nil
	}
	for _, f := range contents {
		name := f.Name()
		if (f.Mode() & os.ModeSymlink) == 0 {
			continue
		}
		path, err := os.Readlink(fp.Join(sysClassNet, name))
		if err != nil {
			log.Logf("err reading link %s: %s\n", name, err)
			continue
		}
		if !strings.Contains(path, "virtual") {
			i, err := net.InterfaceByName(name)
			nic := Nic{name, nil, nil}
			if err == nil {
				nic.mac = i.HardwareAddr
			}
			nics = append(nics, nic)
		}
	}
	return
}

//return array of paths to queues
func (nic Nic) Queues(prefix string) (queues []string) {
	// /sys/class/net/*/queues/[rt]x-*
	qpath := fp.Join(sysClassNet, nic.device, "queues")
	contents, err := ioutil.ReadDir(qpath)
	if err != nil {
		log.Logf("err reading queues: %s\n", err)
		return nil
	}
	for _, q := range contents {
		if strings.HasPrefix(q.Name(), prefix) {
			queues = append(queues, fp.Join(qpath, q.Name()))
		}
	}
	return
}
