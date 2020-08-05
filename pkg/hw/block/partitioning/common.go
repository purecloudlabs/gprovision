// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

// Package partitioning allows creation of MBR and GPT partition
// tables and partitions, DESTROYING ANY EXISTING DATA.
//
// Note that it does *not* support:
//   * conversion between MBR & GPT,
//   * resizing existing partitions,
//   * adding partitions to an existing table,
//   * specifying gaps,
//   * etc.
package partitioning

import (
	"fmt"
	"os/exec"

	"github.com/purecloudlabs/gprovision/pkg/hw/uefi"
	"github.com/purecloudlabs/gprovision/pkg/log"
)

type Partitioner interface {
	Commit() error                                               //write changes to disk
	Add(sizeMegs uint64, ptype partType, boot bool, name string) //add a partition
}

//determines best type of partition table to use, returns a Partitioner to do so
func NewPTable(dev string) Partitioner {
	if uefi.BootedUEFI() {
		return NewGpt(dev)
	}
	return NewMbr(dev)
}

type partType int

const (
	Unused partType = iota
	FAT32
	NTFS
	Linux
	LinuxRaid
	ESP
)

func (t partType) String() string {
	switch t {
	case Unused:
		return "unused partition"
	case FAT32:
		return "fat32 partition"
	case NTFS:
		return "ntfs partition"
	case Linux:
		return "linux partition"
	case LinuxRaid:
		return "linux raid partition"
	case ESP:
		return "EFI system (boot) partition"
	}
	return "partition type out of range"
}

type Partition struct {
	sizeMegs uint64 //a size of 0 indicates "use all available space"
	boot     bool
	ptype    partType
	name     string
}

func (p Partition) String() string {
	size := "unlimited"
	if p.sizeMegs != 0 {
		size = fmt.Sprintf("%dMB", p.sizeMegs)
	}
	return fmt.Sprintf("Partition: name='%s' size=%s boot=%t type=%s", p.name, size, p.boot, p.ptype)
}

func List(dev string) string {
	list := exec.Command("fdisk", "-l", dev)
	out, err := list.CombinedOutput()
	if err != nil {
		log.Logf("executing %v: error %s\nout:\n%s", list.Args, err, out)
		return ""
	}
	return string(out)
}
