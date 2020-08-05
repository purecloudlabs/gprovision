// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package partitioning

import (
	"fmt"
	"os/exec"

	"github.com/purecloudlabs/gprovision/pkg/log"
)

type gpt struct {
	device     string
	partitions []*Partition
	committed  bool
}

var _ Partitioner = &gpt{}

var gptTypes map[partType]uint16

func init() {
	gptTypes = make(map[partType]uint16)
	gptTypes[Unused] = 0x00      //"00000000-0000-0000-0000-000000000000"
	gptTypes[FAT32] = 0x0c00     //"EBD0A0A2-B9E5-4433-87C0-68B6B72699C7"
	gptTypes[NTFS] = 0x0700      //"EBD0A0A2-B9E5-4433-87C0-68B6B72699C7"
	gptTypes[Linux] = 0x8300     //"0FC63DAF-8483-4772-8E79-3D69D8477DE4"
	gptTypes[LinuxRaid] = 0xfd00 //"A19D880F-05FC-4D3B-A006-743F0F84911E"
	gptTypes[ESP] = 0xef00       //"C12A7328-F81F-11D2-BA4B-00A0C93EC93B"
}

func NewGpt(dev string) Partitioner {
	return &gpt{device: dev}
}

func (g *gpt) Commit() error {
	log.Logf("Committing GPT partition table to %s:", g.device)
	if g.committed {
		log.Logf("Already committed")
		return nil
	}
	for _, d := range g.partitions {
		log.Logf("  - %s", d)
	}

	//to erase existing gpt and mbr records, must use both --clear and --mbrtogpt
	sgdisk := exec.Command("sgdisk", "--clear", "--mbrtogpt")
	args := g.assembleArgs()
	sgdisk.Args = append(sgdisk.Args, args...)
	sgdisk.Args = append(sgdisk.Args, g.device)
	out, err := sgdisk.CombinedOutput()
	if err != nil {
		log.Logf("executing %s: %s\nout %s", sgdisk.Args, err, string(out))
		return err
	}
	g.committed = true
	return nil
}
func (g *gpt) Add(sizeMegs uint64, ptype partType, boot bool, name string) {
	if g.committed {
		log.Fatalf("cannot add partition after partitions are written to disk")
	}
	p := &Partition{sizeMegs: sizeMegs, boot: boot, ptype: ptype, name: name}
	g.partitions = append(g.partitions, p)
}

func (g *gpt) assembleArgs() (args []string) {
	for i, p := range g.partitions {
		if p.boot != (p.ptype == ESP) {
			log.Logf("WARNING: UEFI always only boots ESP partitions; mismatch between boot flag and ptype")
		}
		var size string
		if p.sizeMegs > 0 {
			size = fmt.Sprintf("+%dM", p.sizeMegs)
		}
		args = append(args, fmt.Sprintf("--new=%d::%s", i+1, size))
		args = append(args, fmt.Sprintf("--typecode=%d:%04x", i+1, gptTypes[p.ptype]))
		if p.name != "" {
			args = append(args, fmt.Sprintf("--change-name=%d:%s", i+1, p.name))
		}
	}
	return
}
