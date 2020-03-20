// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package raid

import (
	"bytes"
	"encoding/binary"
	"errors"

	//"fmt"
	"gprovision/pkg/hw/ioctl"
	"gprovision/pkg/log"
	"io"
)

const (
	ddfAnchorSize = 512
)

var ddfHeaderSig []byte
var EBLKSSZ error
var EReadSize error
var ELbaVal error

func init() {
	// 0xde11de11 is big-endian representation, as spec'd by SNIA
	ddfHeaderSig = []byte{0xde, 0x11, 0xde, 0x11}
	EBLKSSZ = errors.New("Bad sector size")
	EReadSize = errors.New("Bad read size")
	ELbaVal = errors.New("Bad LBA value")
}

type lba uint64

func (d *Device) detectDDF(buf []byte) bool {
	if buf == nil || len(buf) < ddfAnchorSize {
		panic("nil or short buffer!")
	}

	err := d.readSize()
	if err != nil {
		return false
	}
	anchorStart := int64(d.devSize - ddfAnchorSize)
	buf = buf[len(buf)-ddfAnchorSize:]
	//in addition to sig, could validate checksum... for now, we don't
	if hasSig(buf, ddfHeaderSig) {
		//now find pri/sec offsets, check them
		priLba, secLba := findOffsets(buf)
		pri, err := d.lba2byte(priLba)
		if err == nil && pri >= anchorStart {
			err = ELbaVal
		}
		if err != nil {
			log.Logf("pri lba err %s, priLba=%d, pri=%d", err, priLba, pri)
			return false
		}

		//read primary header
		buf, err = d.AlignedRead(pri, ddfAnchorSize)

		if err == nil && len(buf) != ddfAnchorSize {
			err = EReadSize
		}
		if err != nil {
			log.Logf("pri read err %s", err)
			return false
		}
		if !hasSig(buf, ddfHeaderSig) {
			log.Logf("pri header bad sig")
			return false
		}
		if secLba == 0xffffffffffffffff {
			log.Logf("pri header has good sig; no sec header")
			return true
		}
		sec, err := d.lba2byte(secLba)
		if err == nil && sec >= anchorStart {
			err = ELbaVal
		}
		if err != nil {
			log.Logf("sec lba err %s", err)
			return false
		}
		//read secondary header
		buf, err = d.AlignedRead(sec, ddfAnchorSize)
		if err == nil && len(buf) != ddfAnchorSize {
			err = EReadSize
		}

		if err != nil {
			log.Logf("sec lba read err %s", err)
			return false
		}
		if hasSig(buf, ddfHeaderSig) {
			log.Log("found pri and sec ddf header signatures")
			return true
		}
		log.Logf("sec header bad sig - len(buf)=%d", len(buf))

	}
	return false
}

func (d *Device) saveDDF() (err error) {
	anchor, err := d.AlignedRead(-1*ddfAnchorSize, ddfAnchorSize)

	if len(anchor) != ddfAnchorSize && err == nil {
		err = io.ErrShortWrite
	}
	if err != nil {
		return err
	}

	priLba, secLba := findOffsets(anchor)
	var recordStart int64

	// We don't do a sparse read of the data. Non-sparse _could_ leak data.
	// However, the only way for user data to get written in ddf area would be
	// for someone to compromise the device and deliberately write there. If
	// they do that, what is stopping them from neutering the data erase
	// functionality?

	if secLba > priLba { //takes care of the case where secLba == 0xfff...ff (i.e. no secondary)
		recordStart, err = d.lba2byte(priLba)
	} else {
		recordStart, err = d.lba2byte(secLba)
	}
	if err != nil {
		return err
	}
	if d.devSize == 0 {
		panic("devSize uninitialized!")
	}

	bufSiz := int(d.devSize - uint64(recordStart))
	d.arrayMetadata, err = d.AlignedRead(int64(recordStart), int64(bufSiz))
	if len(d.arrayMetadata) < bufSiz {
		return EReadSize
	}
	//log.Logf("ddf backup of %s: bufSiz=%d, devSize=%d, recordStart=%d, actual buffer len=%d",
	//	d.dev, bufSiz, d.devSize, recordStart, len(d.arrayMetadata))
	return
}

func (d *Device) restoreDDF() (err error) {
	off := int64(-1 * len(d.arrayMetadata))
	err = d.AlignedWrite(off, d.arrayMetadata)
	return
}

//find primary and secondary LBA offsets and convert from LBA into bytewise offsets (*512)
func findOffsets(anchor []byte) (pri, sec lba) {
	//field sizes: 4, 4, 24, 8, 4, 4, 1, 1, 1, 13, 32, 8(pri), 8(sec), ...
	//pri starts at 96
	off := 96
	pri = lba(binary.BigEndian.Uint64(anchor[off : off+8]))

	//sec follows immediately
	off += 8
	sec = lba(binary.BigEndian.Uint64(anchor[off : off+8]))
	return
}

func hasSig(buf, sig []byte) bool {
	l := len(sig)
	if len(buf) < l {
		log.Logf("hasSig: short buffer. l=%d, len=%d", l, len(buf))
		return false
	}
	return bytes.Equal(buf[:l], sig)
}

func endiannessSwap(buf ...byte) (rb []byte) {
	i := 0
	l := len(buf)

	rb = make([]byte, l)
	for i < l {
		rb[i] = buf[l-i-1]
		i++
	}
	return
}

//convert an LBA offset into a byte-offset, using device logical sector size
func (d *Device) lba2byte(offs lba) (offsb int64, err error) {
	if d.sectorSize == 0 {
		d.sectorSize, err = ioctl.BlkGetSectorSize(d.fd)

		if err != nil {
			log.Logf("ioctl BLKSSZGET error %s for %s", err, d.dev)
			return
		}
		if d.sectorSize%512 != 0 {
			log.Logf("bad sector size %d (not a multiple of 512)", d.sectorSize)
			err = EBLKSSZ
		}
	}
	offsb = int64(d.sectorSize) * int64(offs)
	return
}
