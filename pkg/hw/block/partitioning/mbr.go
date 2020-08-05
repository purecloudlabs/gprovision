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
	"strings"

	"github.com/purecloudlabs/gprovision/pkg/log"
)

type mbr struct {
	device     string
	partitions []*Partition
	committed  bool
}

var _ Partitioner = &mbr{}

var mbrTypes map[partType]byte

func init() {
	mbrTypes = make(map[partType]byte)
	mbrTypes[Unused] = 0
	mbrTypes[FAT32] = 0x0c
	mbrTypes[NTFS] = 0x07
	mbrTypes[Linux] = 0x83
	mbrTypes[LinuxRaid] = 0xfd
	mbrTypes[ESP] = 0xef
}
func NewMbr(dev string) Partitioner {
	return &mbr{device: dev}
}

func (m *mbr) Commit() error {
	log.Logf("Committing MBR partition table to %s:", m.device)
	if m.committed {
		log.Logf("Already committed")
		return nil
	}
	for _, d := range m.partitions {
		log.Logf("  - %s", d)
	}

	sfdisk := exec.Command("sfdisk", "--wipe", "always", "--wipe-partitions", "always", m.device)
	sfdisk.Stdin = strings.NewReader(m.commands())
	out, err := sfdisk.CombinedOutput()
	if err != nil {
		log.Logf("executing %s: %s\n===\nin: %s\n===\nout: %s", sfdisk.Args, err, m.commands(), string(out))
		return err
	}
	m.committed = true
	return nil
}

func (m *mbr) Add(sizeMegs uint64, ptype partType, boot bool, name string) {
	if m.committed {
		log.Fatalf("cannot add partition after partitions are written to disk")
	}
	if boot && (len(m.partitions) > 3 || m.haveBootable()) {
		log.Logf("ignoring boot flag for partition #%d", len(m.partitions)+1)
		boot = false
	}
	p := &Partition{sizeMegs: sizeMegs, boot: boot, ptype: ptype, name: name}
	m.partitions = append(m.partitions, p)
}

func (m *mbr) commands() (cmds string) {
	for _, p := range m.partitions {
		var size, bootable string
		if p.sizeMegs > 0 {
			size = fmt.Sprintf("%dM", p.sizeMegs)
		}
		if p.boot {
			bootable = "*"
		}
		cmds += fmt.Sprintf(",%s,%x,%s\n", size, mbrTypes[p.ptype], bootable)
	}
	return
}

func (m *mbr) haveBootable() bool {
	for _, p := range m.partitions {
		if p.boot {
			return true
		}
	}
	return false
}
