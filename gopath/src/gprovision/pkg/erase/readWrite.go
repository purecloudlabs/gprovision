// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package erase

import (
	"bytes"
	"fmt"
	"gprovision/pkg/common"
	"gprovision/pkg/erase/raid"
	"gprovision/pkg/log"
	"syscall"
	"time"
)

const (
	oneK = 1024
	oneM = oneK * oneK
	oneG = 1024 * oneM
)

var prepPattern string

func init() {
	prepPattern = fmt.Sprintf("~~erase begins %s~~", time.Now())
}

//write a pattern into the buffer
//3 patterns - 0x55, 0xAA, 0x00
func fillPattern(buf []byte, p int) {
	switch p {
	case 0:
		for i := range buf {
			buf[i] = 0x55
		}
	case 1:
		for i := range buf {
			buf[i] = 0xAA
		}
	default:
		for i := range buf {
			buf[i] = 0x00
		}
	}
}

//prepare the disk - write a pattern in certain places. (every 100M?)
func prepare(d *raid.Device, recov common.Pather) {
	var err error
	name := d.Dev()
	name = name[len(name)-3:]

	pattern := []byte(prepPattern)
	var offs int64
	patternWriteCount := 0
	for {
		err = d.AlignedWrite(offs, pattern)
		if err != nil {
			err = d.AlignedWrite(offs, pattern)
			if err != nil {
				log.Logf("prepare %s: failed to write, err==%s", name, err)
				unrecoverableFailure(recov, true)
			}
		}
		//log.Logf("wrote pattern at %s:%d (iter %d)", name, offs, patternWriteCount)

		patternWriteCount++
		offs += oneG

		if !d.InRange(offs, len(pattern)) {
			log.Logf("%s:wrote pattern in %d places. stopping with offs=%d", name, patternWriteCount, offs)
			break
		}
	}
	syscall.Sync()
	writtenCount := countPrepPattern(d, recov)
	if writtenCount != patternWriteCount {
		log.Logf("prepare %s: writtenCount=%d, patternWriteCount=%d", name, writtenCount, patternWriteCount)
		unrecoverableFailure(recov, true)
	}
}

//count the number of occurences of the prep pattern
//used to ensure that prepare() worked, as well as in verify()
func countPrepPattern(d *raid.Device, recov common.Pather) int {
	var err error
	name := d.Dev()
	name = name[len(name)-3:]

	pattern := []byte(prepPattern)
	var offs int64
	patternCount := 0
	var buf []byte
	for {
		buf, err = d.AlignedRead(offs, int64(len(pattern)))
		if err != nil {
			unrecoverableFailure(recov, true)
		}
		if bytes.Equal(buf, pattern) {
			//log.Logf("pattern match at %s:%d (iter %d). len(buf)==%d,\n     buf==%.20s", name, offs, offs/oneG, len(buf), buf)
			patternCount++
		} else {
			log.Logf("no pattern at %s:%d (iter %d). len(buf)==%d,\n     buf==%.20s", name, offs, offs/oneG, len(buf), buf)
		}
		if !d.InRange(offs+oneG, len(pattern)) {
			log.Logf("%s: found pattern in %d places. stopping with offs=%d", name, patternCount, offs)
			break
		}
		offs += oneG
		if err != nil {
			log.Logf("error counting pattern on %s: %s", name, err)
			break
		}
	}
	return patternCount
}

//verify erasure - check if the pattern written by prepare exists anywhere
//if it does, this is an unrecoverable failure
func verify(d *raid.Device, recov common.FS) {
	if _, err := d.Open(); err != nil {
		log.Logf("open for verify: %s", err)
	}
	writtenCount := countPrepPattern(d, recov)
	if writtenCount != 0 {
		log.Logf("%s: writtenCount=%d, want 0!", d.Dev(), writtenCount)
		unrecoverableFailure(recov, true)
	}
}
