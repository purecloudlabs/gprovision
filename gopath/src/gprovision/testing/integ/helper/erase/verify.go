// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package main

import (
	"errors"
	"gprovision/pkg/appliance"
	"gprovision/pkg/log"
	"gprovision/pkg/recovery/disk"
	"io"
	"os"
	"syscall"
)

var EPatFound = errors.New("pattern found after erasure")

//verify that erase actually erased the drive
func verify(plat *appliance.Variant, disks []*disk.Disk) {
	tgt := disks[0]
	err := findPatterns(tgt)
	if err != nil {
		log.Fatalf("error %s reading patterns", err)
	} else {
		log.Log("patterns not present - success. exiting.")
	}
}
func findPatterns(d *disk.Disk) error {
	blk, err := os.OpenFile(d.Device(), os.O_RDONLY|syscall.O_DIRECT, 0)
	if err != nil {
		return err
	}
	defer blk.Close()
	siz := d.SizeBytes()
	offsets := offsetSeq(siz)
	//determine count/offsets
	for _, o := range offsets {
		err = readPat(blk, d.Device(), o)
		if err != nil {
			return err
		}
	}
	return nil
}

func readPat(f *os.File, dev string, offs int64) error {
	_, err := f.Seek(offs, io.SeekStart)
	if err != nil {
		return err
	}
	buf := make([]byte, oneM)
	_, err = f.Read(buf)
	if err != nil {
		return err
	}

	//should be zeros?
	var found bool
	for i, b := range buf {
		if b != 0 {
			log.Logf("o=%d, d=%s: non-zero found at +%x: %x", offs, dev, i, b)
			found = true
		}
	}
	if found {
		return EPatFound
	}

	return nil
}
