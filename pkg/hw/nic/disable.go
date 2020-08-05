// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package nic

import (
	"io/ioutil"
	"os"
	"sync"
	"time"

	"github.com/purecloudlabs/gprovision/pkg/hw/ioctl"
	"github.com/purecloudlabs/gprovision/pkg/log"
)

// Set nic state down
func (nic Nic) Down() {
	err := ioctl.SetNicState(nic.device, false)
	if err != nil {
		log.Logf("failed to set interface %s down: %s", nic.device, err)
	}
}

// Set nic state up
func (nic Nic) Up() {
	err := ioctl.SetNicState(nic.device, true)
	if err != nil {
		log.Logf("failed to set interface %s up: %s", nic.device, err)
	}
}

// Disables the given nic until reboot.
//
// WARNING: if multiple ports are to be disabled, disable the one with
// highest index first; otherwise the wrong port(s) will be disabled.
func DisableByIndex(i int, allowedPrefixes [][]uint8) bool {
	list := SortedList(allowedPrefixes)
	if i >= len(list) {
		log.Logf("index out of range, cannot disable #%d", i)
		return false
	}
	return list[i].Disable()
}

var disabledNicCount int
var disabledNicMtx sync.Mutex

// Returns number of nics that have been disabled. Note, only counts disables
// by this process, not by all processes on system.
func DisabledNics() int {
	disabledNicMtx.Lock()
	defer disabledNicMtx.Unlock()
	return disabledNicCount
}

// Sets nic state down and detaches from driver, causing the nic to disappear
// until reboot.
func (nic Nic) Disable() (success bool) {
	/* this one doesn't seem to work:
	* echo "eth3" > /sys/bus/pci/drivers/bnx2/unbind
	* -- https://www.redhat.com/archives/rhl-list/2008-February/msg01639.html
	*
	* this one works:
	* echo 1 > /sys/devices/pci0000:00/0000:00:1c.0/0000:09:00.0/remove
	* http://www.6by9.net/using-linux-sys-to-disable-ethernet-hardware-devices/

	  ip link set dev ... down
	  /sys/class/net/.../device/remove
	*/
	defer func() {
		if success {
			disabledNicMtx.Lock()
			defer disabledNicMtx.Unlock()
			disabledNicCount++
		}
	}()
	nic.Down()

	err := ioutil.WriteFile("/sys/class/net/"+nic.device+"/device/remove", []byte("1"), 0600)
	if err != nil {
		log.Logf("failed to remove device %s: %s", nic.device, err)
	}
	//wait a bit, check if interface still exists
	time.Sleep(time.Millisecond * 100)
	_, err = os.Stat("/sys/class/net/" + nic.device)
	if err != nil {
		if os.IsNotExist(err) {
			log.Logf("disabled %s", nic.device)
			success = true
		} else {
			log.Logf("attempted to disable nic %s, but encountered error %s", nic.device, err)
		}
	} else {
		log.Logf("attempted to disable nic %s, but file still exists", nic.device)
	}
	return
}
