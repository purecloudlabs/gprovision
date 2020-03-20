// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

//Package disk handles logical disks, filesystems, and boot entries for the
//purposes of factory restore.
package disk

import (
	"fmt"
	"gprovision/pkg/appliance"
	"gprovision/pkg/common/strs"
	"gprovision/pkg/hw/block"
	"gprovision/pkg/hw/block/partitioning"
	"gprovision/pkg/hw/uefi"
	"gprovision/pkg/log"
	"io"
	"os"
	"os/exec"
	fp "path/filepath"
	"syscall"
)

type Disk struct {
	identifier string //sda, sr0, nbd3, etc
	size       int64
	target     int //partition number to use when creating raid array (or root fs, on non-raid plats)
	numParts   int //total number of partitions
}

func (d Disk) SizeBytes() int64 {
	return d.size
}

//find the raw disk(s) we'll partition and install to
func FindTargets(platform *appliance.Variant) (disks []*Disk) {
	//stop any raid arrays
	stopArr := exec.Command("mdadm", "--stop", "--scan")
	out, err := stopArr.CombinedOutput()
	if err != nil {
		log.Logf("error stopping array(s): %s\nout:\n%s\n", err, out)
	}
	devs := block.Devices()
	var candidates dlist
	for _, dev := range devs {
		candidates = append(candidates, &Disk{fp.Base(dev.Name), int64(dev.Size), -1, 0})
	}
	wantSize := platform.DiskSize()
	wantNum := platform.DataDisks()
	disks = candidates.filter(wantSize, wantNum, platform.DiskSetTol(), platform.DiskTgtTol())
	if len(disks) != wantNum {
		var arrayDesc string
		if platform.HasRaid() {
			arrayDesc = fmt.Sprintf(" for RAID%d array", platform.RaidLevel())
		}
		msg := fmt.Sprintf("Need %d disk(s)%s of size %d, found %d. Candidates:\n%s\n", wantNum, arrayDesc, wantSize, len(candidates), candidates)
		log.Fatalf(msg)
	}
	return
}

func DiskFromDev(dev block.BlockDev) *Disk {
	return &Disk{
		identifier: fp.Base(dev.Name),
		size:       int64(dev.Size),
		target:     -1,
	}
}

type dlist []*Disk

func (dl dlist) Len() int           { return len(dl) }
func (dl dlist) Less(i, j int) bool { return dl[i].size < dl[j].size }
func (dl dlist) Swap(i, j int)      { dl[i], dl[j] = dl[j], dl[i] }
func (dl dlist) String() (s string) {
	if len(dl) == 0 {
		return "(nil)"
	}
	s = "["
	for i, d := range dl {
		s += fmt.Sprintf("%d: %s=%d, ", i, d.identifier, d.size)
	}
	s = s[:len(s)-2] //trailing chars
	s += "]"
	return
}

//given a list of disks, return the ones which most closely match the target size
//count - must return this many disks or nil
//setTolPct - size tolerance amongst set of disks in 'out'
//tgtTolPct - allowed deviation from tgtSize
func (in dlist) filter(tgtSize uint64, count int, setTolPct, tgtTolPct uint64) (out dlist) {
	if count > len(in) {
		return
	}
	//make a copy so as to not overwrite 'in'
	workingList := make(dlist, len(in))
	copy(workingList, in)
	//iterate through dl, moving elements to out.
	for len(out) < count {
		if len(workingList) == 0 {
			return nil
		}
		closest := workingList.closest(tgtSize)
		out = append(out, workingList[closest])
		workingList = append(workingList[:closest], workingList[closest+1:]...)
		/* If size difference between elements is too great, discard element 0.
		   Because the list is ordered, there can be no better match for that element.
		*/
		if len(out) > 1 && !block.SizeToleranceMatch(uint64(out[0].size), uint64(out[1].size), setTolPct) {
			out = out[1:]
		}
	}
	if len(out) > count {
		out = out[:count]
	}
	/* 'out' is the best possible set given the constraints.
	   If any element exceeds tgtTol, there is no viable set.
	   Check tolerance with last (farthest) element.
	*/
	if !block.SizeToleranceMatch(uint64(out[len(out)-1].size), tgtSize, tgtTolPct) {
		log.Logf("scoreDisks: element size %d exceeds target tolerance (%d ± %d%%) - not enough disks to proceed", out[len(out)-1].size, tgtSize, tgtTolPct)
		out = nil
	}
	var largest, smallest int64
	for i, d := range out {
		if i == 0 {
			largest = d.size
			smallest = d.size
			continue
		}
		if d.size > largest {
			largest = d.size
		}
		if d.size < smallest {
			smallest = d.size
		}
	}
	if !block.SizeToleranceMatch(uint64(smallest), uint64(largest), setTolPct) {
		log.Logf("scoreDisks: %d, %d out of range (± %d%%)", largest, smallest, setTolPct)
		out = nil
	}
	return
}

//returns index of element closest to tgtSize
func (dl dlist) closest(tgtSize uint64) (idx int) {
	iabs := func(v int64) int64 {
		if v > 0 {
			return v
		}
		return -1 * v
	}
	var delta int64 = -1
	for i := range dl {
		dist := iabs(int64(tgtSize) - dl[i].size)
		if delta == -1 || dist < delta {
			delta = dist
			idx = i
		}
	}
	return
}

func (d Disk) Device() string {
	return "/dev/" + d.identifier
}

func (d Disk) TargetDev() string {
	return fmt.Sprintf("/dev/%s%d", d.identifier, d.target)
}

func (d Disk) Valid() bool {
	return len(d.identifier) != 0
}

// clears bytes at beginning or end of drive. Whence should be 'disk.SeekStart' or 'disk.SeekEnd'. NOT a substitute for secure erase.
func (d *Disk) Zero(megs uint, whence int) {
	zero("/dev/"+d.identifier, megs, whence)
}
func zero(dev string, megs uint, whence int) {
	if whence != io.SeekStart && whence != io.SeekEnd {
		panic(fmt.Sprintf("Zero(%d, %d): bad whence", megs, whence))
	}
	blk, err := os.OpenFile(dev, os.O_WRONLY|syscall.O_DIRECT, 0)
	if err != nil {
		//log, but proceed anyway?
		log.Msg("Array initialization may fail")
		log.Logf("Failed to open device %s: %s\n", dev, err)
		return
	}
	defer blk.Close()
	oneM := 1024 * 1024
	zeros := make([]byte, oneM)
	if whence == io.SeekEnd {
		if _, err := blk.Seek(int64(-1*int(megs)*oneM), io.SeekEnd); err != nil {
			log.Logf("seeking: %s", err)
		}
	}
	for i := 0; i < int(megs); i++ {
		if _, err := blk.Write(zeros); err != nil {
			log.Logf("writing zeros: %s", err)
		}
	}
}

//partition a disk. small boot partition, remainder for raid
//also zero beginning and end of disk so raid controller will ignore it if the controller somehow gets re-enabled
func (d *Disk) Partition(platform *appliance.Variant) error {
	d.Zero(250, io.SeekStart)
	//SNIA DDF and iMSM require metadata be at end of device, so this will wipe it; GPT uses beginning and end so it gets wiped as well.
	d.Zero(100, io.SeekEnd)

	mainPartType := partitioning.Linux
	if platform.HasRaid() {
		mainPartType = partitioning.LinuxRaid
	}
	pt := partitioning.NewPTable("/dev/" + d.identifier)
	if uefi.BootedUEFI() {
		//for uefi, we always only boot from the usb key (???)
		pt.Add(0, mainPartType, false, strs.PriVolName())
		d.target = 1 // target = main volume, 1st partition in this case
		d.numParts = 1
	} else {
		pt.Add(200, partitioning.Linux, false, "g4d")
		pt.Add(0, mainPartType, false, strs.PriVolName())
		d.target = 2 // target = main volume, 2nd partition in this case
		d.numParts = 2
	}
	return pt.Commit()
}

//partition recovery device
func PartitionRecovery(d *Disk) error {
	pt := partitioning.NewPTable("/dev/" + d.identifier)
	if uefi.BootedUEFI() {
		log.Logf("Partitioning recovery drive with GPT scheme")
		pt.Add(200, partitioning.ESP, true, "ESP")
		pt.Add(0, partitioning.Linux, false, "recovery")
		d.numParts = 2
	} else {
		log.Logf("Partitioning recovery drive with MBR scheme")
		pt.Add(0, partitioning.Linux, true, "recovery")
		d.numParts = 1
	}
	return pt.Commit()
}
