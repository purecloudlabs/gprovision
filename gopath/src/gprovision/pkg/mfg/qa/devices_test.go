// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package qa

import (
	"os"
	"testing"
)

//func listDevs(t string) (devs []string)
func TestListPCIDevs(t *testing.T) {
	devs, err := listDevs(pcidevs)
	if err != nil {
		t.Errorf("%s", err)
	}
	for _, d := range devs {
		t.Logf("%v\n", d)
	}
}

//func readDev(path string) (vendor, device, class uint64, err error)
func TestReadPCIDev(t *testing.T) {
	devs, err := listDevs(pcidevs)
	if err != nil {
		t.Errorf("%s", err)
	}
	v, d, c, err := readDev(devs[0])
	if err != nil {
		t.Errorf("%s", err)
	}
	t.Logf("v=0x%x, d=0x%x, c=0x%x", v, d, c)
}

//func listDevs(t string) (devs []string)
func TestListUSBDevs(t *testing.T) {
	_, err := os.Stat(usbdevs)
	if err != nil && os.IsNotExist(err) {
		if _, found := os.LookupEnv("JENKINS_NODE_COOKIE"); found {
			t.Skip("jenkins lacks USB devices")
		}
	}
	devs, err := listDevs(usbdevs)
	if err != nil {
		t.Errorf("%s", err)
	}
	for _, d := range devs {
		t.Logf("%v\n", d)
	}
}

//func readDev(path string) (vendor, device, class uint64, err error)
func TestReadUSBDev(t *testing.T) {
	_, err := os.Stat(usbdevs)
	if err != nil && os.IsNotExist(err) {
		if _, found := os.LookupEnv("JENKINS_NODE_COOKIE"); found {
			t.Skipf("jenkins lacks USB devices")
		}
	}
	devs, err := listDevs(usbdevs)
	if err != nil {
		t.Errorf("%s", err)
	}
	for _, dev := range devs {
		v, d, c, err := readDev(dev)
		if err == errSkipThisDevice {
			t.Logf("skipping %s", dev)
			continue
		}
		if err != nil {
			t.Errorf("%s", err)
		}
		t.Logf("v=0x%04x, d=0x%04x, c=0x%04x", v, d, c)
	}
}
