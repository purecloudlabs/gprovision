// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package block

import (
	"fmt"
	"gprovision/pkg/hw/ioctl"
	"gprovision/pkg/log"
	"io/ioutil"
	"os"
	fp "path/filepath"
	"strings"
	"syscall"
)

type BlockDev struct {
	Name   string
	Size   uint64
	Model  string
	Vendor string
}

func (b BlockDev) String() string {
	return fmt.Sprintf("Device %s: Vendor=%s, Model=%s, Size=%d", b.Name, b.Vendor, b.Model, b.Size)
}

//return name, size of storage devices
func Devices() (devs []BlockDev) {
	// sys/class/block is a superset of sys/block; it also contains partitions
	names := devices("/sys/block", nil)
	for _, name := range names {
		s, err := ReadSize(name)
		if err != nil {
			log.Logf("error %s for %s", err, name)
			continue
		}
		m, err := ReadModel(fp.Base(name))
		if err != nil {
			log.Logf("error %s for %s", err, name)
			continue
		}
		v, err := ReadVendor(fp.Base(name))
		if err != nil {
			log.Logf("error %s for %s", err, name)
			continue
		}
		devs = append(devs, BlockDev{name, s, m, v})
	}
	return
}

//A function that returns true if the entry is to be kept. Note that syspath
//is likely relative to /sys/class/block.
type DevIncludeFn func(syspath string) bool

func DFiltOnlyUsb(syspath string) bool {
	/*
	   /sys/class/block/sdb1 ->
	   ../../devices/pci0000:00/0000:00:07.1/0000:08:00.3/usb3/3-1/3-1.3/3-1.3:1.0/host9/target9:0:0/9:0:0:0/block/sdb/sdb1
	*/
	return strings.Contains(syspath, "usb")
}

func DFiltOnlyParts(syspath string) bool {
	_, err := os.Stat(fp.Join(syspath, "partition"))
	return err == nil
}

func DFiltOnlyUsbParts(syspath string) bool {
	return DFiltOnlyUsb(syspath) && DFiltOnlyParts(syspath)
}

func devices(sysdir string, include DevIncludeFn) (devs []string) {
	dir, err := ioutil.ReadDir(sysdir)
	if err != nil {
		return
	}
	for _, entry := range dir {
		link, err := os.Readlink(fp.Join(sysdir, entry.Name()))
		if err != nil || strings.Contains(link, "devices/virtual/block") {
			continue
		}
		if include != nil && !include(link) {
			continue
		}
		devs = append(devs, "/dev/"+entry.Name())
	}
	return devs
}

//Return a path for each non-virtual block device. Unlike Devices(), include
//partitions.
func AllBlockDevs() []string {
	return devices("/sys/class/block", nil)
}

//like AllBlockDevs, but uses a filter function to limit the results
func FilterBlockDevs(filter DevIncludeFn) []string {
	return devices("/sys/class/block", filter)
}

//SizeToleranceMatch returns true if actual value is within tolerance of desired value.
//Tolerance is expressed as an integer 1-100, representing a percent.
func SizeToleranceMatch(have, want, tol uint64) bool {
	abs := func(v int64) uint64 {
		if v < 0 {
			v *= -1
		}
		return uint64(v)
	}
	deviation := abs(int64(want) - int64(have))
	limit := want * tol / 100
	return deviation <= limit
}

//use ioctl to find dev size
func ReadSize(dev string) (devSize uint64, err error) {
	fd, err := os.OpenFile(dev, syscall.O_DIRECT|os.O_RDONLY, 0600)
	if err != nil {
		return
	}
	defer fd.Close()
	devSize, err = ioctl.BlkGetSize64(fd)
	if err != nil {
		devSize = 0
	}
	return
}

//given a dev like '/dev/sda', find device model string
//for sata, 'model' file includes vendor as well
func ReadModel(dev string) (m string, err error) {
	//ls -ld /sys/block/sda
	// /sys/block/sda -> /sys/devices/pci0000:00/0000:00:11.0/ata1/host0/target0:0:0/0:0:0:0/block/sda
	// model string in   /sys/devices/pci0000:00/0000:00:11.0/ata1/host0/target0:0:0/0:0:0:0/model
	mfile := fp.Join("/sys/block", dev, "device", "model")
	f, err := ioutil.ReadFile(mfile)
	if err != nil {
		return
	}
	m = strings.TrimSpace(string(f))
	return
}

//for sata, always returns ATA
//for scsi, ???
//for usb, returns actual vendor
func ReadVendor(dev string) (v string, err error) {
	vfile := fp.Join("/sys/block", dev, "device", "vendor")
	f, err := ioutil.ReadFile(vfile)
	if err != nil {
		return
	}
	v = strings.TrimSpace(string(f))
	return
}

//IsDev returns true if given dev represents a physical device
func IsDev(dev string) bool {
	_, err := os.Stat(fp.Join("/sys/class/block", dev, "device"))
	return err == nil
}

//IsPart returns true if given dev is a partition
func IsPart(dev string) bool {
	_, err := os.Stat(fp.Join("/sys/class/block", dev, "partition"))
	return err == nil
}

//PartNum returns the part number of a partition, or empty string
func PartNum(dev string) string {
	data, err := ioutil.ReadFile(fp.Join("/sys/class/block", dev, "partition"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

//PartParent returns the parent block device for the given partition. Does not check if dev is actually a partition.
func PartParent(dev string) string {
	s, _ := os.Readlink(fp.Join("/sys/class/block", dev))
	if s == "" {
		return ""
	}
	p := fp.Dir(s)
	if len(p) < 2 {
		return ""
	}
	return fp.Base(p)
}
