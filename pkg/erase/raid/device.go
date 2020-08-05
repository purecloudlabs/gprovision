// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package raid

/* host RAID metadata formats
 *
 * - Intel Matrix Storage - used on SuperMicro X10SRH-CLN4F
 *   - 1024 bytes (2 sectors) at end of drive
 *   - AF drives - still 1k?
 *   - begins with "Intel Raid ISM Cfg"
 * - SNIA DDF - used on S2400EP (Intel ESRT)
 *   - also at end of drive, but MINIMUM 32 MB required by spec (s2400 uses MORE)
 *   - complex
 *   - Anchor header in last LBA contains LBA pointers for Primary and Secondary headers, each heading a record set
 *   - Anchor, Primary, Secondary headers all begin with 0xDE11DE11
 *   - read pri/sec lba's from anchor, then save all data between min(pri, sec) and end of drive?
 *   - s2400: offset 13fe000 between pri, sec headers
 */

//have vars for expected # of arrays, devs?

/*
 ADVANCED FORMAT DRIVES
 test both s2400 and x10srh with at least one brand/model of AF drive!!!
*/

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"syscall"

	"github.com/purecloudlabs/gprovision/pkg/appliance"
	"github.com/purecloudlabs/gprovision/pkg/hw/ioctl"
	"github.com/purecloudlabs/gprovision/pkg/log"
)

const oneG = 1024 * 1024 * 1024

type raidType int

const (
	unset raidType = iota
	unknown
	msm
	ddf
)

func (r raidType) String() string {
	switch r {
	case unset:
		return "(unset)"
	case unknown:
		return "(unknown)"
	case msm:
		return "Intel Matrix Storage"
	case ddf:
		return "SNIA DDF"
	default:
		panic("Error!")
	}
}

var EUnknownRaidFormat error
var ENoData error
var ETypeMismatch error
var EAlignment error
var EBounds error

var sizeTol float64 = 0.05

func init() {
	EUnknownRaidFormat = errors.New("unknown raid format")
	ENoData = errors.New("no data to write")
	ETypeMismatch = errors.New("array type mismatch")
	EAlignment = errors.New("aligned read failure")
	EBounds = errors.New("IO operation out of bounds")
}

type ReadWriteSeekCloser interface {
	io.Reader
	io.Writer
	io.Seeker
	io.Closer
	Fd() uintptr
}

type Device struct {
	array         *Array
	alignment     int64
	arrayType     raidType
	dev           string
	sectorSize    uint64
	devSize       uint64
	fd            ReadWriteSeekCloser
	arrayMetadata []byte
}

func SetSizeTol(tol float64) {
	if tol <= 0 || tol > .2 {
		panic(fmt.Sprintln("value out of range", tol))
	}
	sizeTol = tol
}

//look through block dev's in /sys/block for candidates, return Device for each
//excludes recovery device
func FindDevices(platform *appliance.Variant) (devices Devices, err error) {
	dir, err := ioutil.ReadDir("/sys/block")
	if err != nil {
		return
	}
	var thr uint64 = 200 * oneG
	if platform.IsPrototype() && platform.RecoveryDevVirt() {
		// for integ tests, impractical to have large root volume
		// plus 9p recovery has no block device
		thr = 0
	}

outer:
	for _, entry := range dir {
		for _, pfx := range []string{"loop", "ram", "zram", "nbd"} {
			if strings.HasPrefix(entry.Name(), pfx) {
				continue outer
			}
		}
		entnm := "/dev/" + entry.Name()
		if platform.CheckRecoveryDev(entnm) == nil {
			log.Logf("FindDevices: %s matches recovery, skipping.", entnm)
			//maybe also check label?
			continue
		}
		d := NewDevice(entnm)

		err = d.DetectRaidType(thr)
		if err == nil {
			devices = append(devices, &d)
		} else {
			loop := strings.HasPrefix(entry.Name(), "loop")
			nbd := strings.HasPrefix(entry.Name(), "nbd")
			if !loop && !nbd {
				log.Logf("FindDevices: skipping %s (err: %s)", entnm, err)
			}
		}
	}
	if len(devices) > 0 {
		err = nil
	}

	return
}

func NewDevice(dev string) (d Device) {
	d.dev = dev
	return
}

func (d *Device) InArray() bool {
	return d.array != nil
}

//returns path in /dev for device
func (d *Device) Dev() string {
	return d.dev
}

//open device as a file
func (d *Device) Open() (ReadWriteSeekCloser, error) {
	var err error
	if d.fd != nil {
		return d.fd, nil
	}
	d.fd, err = os.OpenFile(d.dev, syscall.O_DIRECT|os.O_RDWR, 0600)
	if err != nil {
		d.fd = nil
	}
	return d.fd, err
}
func (d *Device) Close() {
	if d.fd != nil {
		d.fd.Close()
		d.fd = nil
	}
}

//detect raid type
func (d *Device) DetectRaidType(thr uint64) (err error) {
	var readSize int64 = 8192
	_, err = d.Open()
	if err != nil {
		return
	}
	buf, err := d.AlignedRead(-1*readSize, readSize)
	if err == nil && int64(len(buf)) != readSize {
		err = EReadSize
	}
	if err != nil {
		//log.Logf("%s: read %d bytes metadata, err=%s", d.dev, len(buf), err)
		return
	}

	if d.detectIMSM(buf) {
		d.arrayType = msm
		return
	}
	if d.detectDDF(buf) {
		d.arrayType = ddf
		return
	}
	// Differentiate between data disks with unknown format and others, such as
	// factory restore, by checking device size.
	size, _ := d.ReadSize()
	if size > thr {
		d.arrayType = unknown
	}
	//array type defaults to 'unset' - which we'll treat specially when creating arrays
	return

	//do we ever want this error??
	//return EUnknownRaidFormat
}

//load raid array metadata into buffer, in prep for disk erase
func (d *Device) Backup() (err error) {
	d.fd, err = d.Open()
	if err != nil {
		return
	}
	//defer d.fd.Close()
	if d.arrayType == msm {
		return d.saveIMSM()
	}
	if d.arrayType == ddf {
		return d.saveDDF()
	}
	if d.arrayType == unknown {
		//do nothing
		return nil
	}
	//should never get here
	return EUnknownRaidFormat
}

//write data in buffer back to disk
func (d *Device) Restore() (err error) {
	log.Logf("%s: Restoring %d bytes %s metadata", d.dev, len(d.arrayMetadata), d.arrayType)
	d.fd, err = d.Open()
	if err != nil {
		return
	}
	//defer d.fd.Close()
	if d.arrayMetadata == nil || len(d.arrayMetadata) == 0 {
		return ENoData
	}

	if d.arrayType == msm {
		return d.restoreIMSM()
	}
	if d.arrayType == ddf {
		return d.restoreDDF()
	}
	if d.arrayType == unknown {
		//do nothing
		return nil
	}
	//should never get here
	return EUnknownRaidFormat

}

//compare dev size, with tolerance
func sizeMatch(a, b *Device) bool {
	var err error
	if a.devSize == 0 {
		err = a.readSize()
	}
	if b.devSize == 0 && err == nil {
		err = b.readSize()
	}
	if err != nil {
		log.Logf("unable to compare drives %s, %s due to error(s) reading device size", a.dev, b.dev)
		return false
	}

	if a.devSize > b.devSize {
		return float64(a.devSize) <= float64(b.devSize)*(1.0+sizeTol)
	}
	return float64(b.devSize) <= float64(a.devSize)*(1.0+sizeTol)
}

//use ioctl to find dev size
func (d *Device) readSize() (err error) {
	d.devSize, err = ioctl.BlkGetSize64(d.fd)
	if err != nil {
		log.Logf("ioctl BLKGETSIZE64 error %s for %s", err, d.dev)
		d.devSize = 0
	}
	return
}
func (d *Device) ReadSize() (uint64, error) {
	var err error
	if d.devSize == 0 {
		err = d.readSize()
	}
	return d.devSize, err
}

//return last used alignment
func (d *Device) Alignment() int64 {
	return d.alignment
}

//check if a read/write with offset 'at' and len 'size' is in range
func (d *Device) InRange(at int64, size int) bool {
	_, err := d.ReadSize()
	if err != nil {
		return false
	}
	if at < 0 {
		at = int64(d.devSize) + at
	}
	ap, off := align(at+int64(size), d.alignment)
	return ap+off <= int64(d.devSize)
}

//return 'at' or a more negative value that is evenly divisible by 'how'
//sign of 'at' and 'res' will match
func align(at, how int64) (res, off int64) {
	if at%how == 0 {
		return at, 0
	}
	if at > 0 && at < how {
		return 0, at
	}
	if at < 0 {
		off = how + (at % how)
	} else {
		off = at % how
	}
	res = at - off
	return
}

//read from device, using oversized aligned reads to satisfy device requirements when at/size are not aligned
func (d *Device) AlignedRead(at, size int64) ([]byte, error) {
	buf, _, o1, o2, err := d.rawAlignedRead(at, size)
	if err != nil {
		return nil, err
	}
	//return portion of buffer containing requested data
	return buf[o1:o2], err
}

//perform an aligned read, returning various values for the calling function to use
//check device bounds, report bad 'at' values as different error
func (d *Device) rawAlignedRead(at, reqSize int64) (buf []byte, begin, offset1, offset2 int64, err error) {
	abs := func(v int64) uint64 {
		if v > 0 {
			return uint64(v)
		}
		return uint64(-1 * v)
	}
	if _, err := d.ReadSize(); err != nil {
		log.Logf("aligned read: %s", err)
	}
	if abs(at) > d.devSize {
		//fmt.Printf("err - at=%d,abs=%d,s=%d",at,abs(at),d.devSize)
		err = EBounds
		return
	}
	if d.alignment == 0 {
		d.alignment = 512
	}

	//where to stop? 4k? 8k? 16k?
	var maxAlign int64 = 16 * 1024
	//ensure buf is at least 'size' _and_ is padded
	//read size usually has to be aligned as well, so maxAlign + size isn't enough
	buf = make([]byte, (maxAlign*2)+reqSize)
	for d.alignment <= maxAlign {
		//find alignment point at or before 'at'
		begin, offset1 = align(at, d.alignment)
		//on success, buf[offset1:offset2] will be the requested data
		offset2 = offset1 + reqSize
		if begin < 0 {
			begin, err = d.fd.Seek(begin, 2)
		} else {
			_, err = d.fd.Seek(begin, 0)
		}
		//reading too much at the end will result in an error, so we'll use a smaller chunk of the buffer
		//begin+offset1 is same location as 'at', except always positive - never relative to EOF
		readEnd, o := align(begin+offset1+reqSize, d.alignment)
		readEnd -= begin
		if o > 0 {
			readEnd += d.alignment
		}
		//if err != nil {
		//	log.Logf("seek error in alignedRead. at=%d, size=%d, align=%d, err=%s", at, size, d.alignment, err)
		//}
		var totalRead int
		if err == nil {
			//read into buffer
			totalRead, err = d.fd.Read(buf[:readEnd])
		}
		//if err != nil {
		//	fmt.Printf("aligned read: seek=%d, totalRead=%d, err=%s, len(buf)=%d\n",
		//		begin, totalRead, err, len(buf))
		//}
		if err == io.EOF && int64(totalRead)-offset1 >= reqSize {
			//if true, the EOF was in the padding after the important data... so we don't care
			err = nil
		}
		//if err == nil && int64(n)-off < size {
		//	fmt.Fprintf(os.Stderr, "AlignedRead: error. n=%d, off=%d, size=%d, align=%d\n", n, off, size, alignment)
		//	return nil, EAlignment
		//}

		//if the read was truncated, ensure our offset remains within bounds
		if offset2 > int64(totalRead) {
			offset2 = int64(totalRead)
		}
		//fmt.Fprintf(os.Stderr, "AlignedRead: n=%d, off=%d, size=%d, align=%d, l=%d\n", n, off, size, alignment, l)
		if err == nil {
			buf = buf[:totalRead]
			return
		}
		d.alignment *= 2
		//fmt.Fprintf(os.Stderr, "retrying read with alignment %d\n", alignment)
	}
	/* if we get here, it's an error. reset the alignment
	 * so additional operations won't immediately fail */
	d.alignment = 0
	buf = nil
	if err == nil {
		err = EAlignment
	}
	return
}

//read, modify, write
func (d *Device) AlignedWrite(at int64, data []byte) error {
	size := int64(len(data))
	buf, begin, offset1, offset2, err := d.rawAlignedRead(at, size)
	if err == nil && offset2-offset1 < size {
		err = io.ErrShortWrite
	}
	var n int
	if err == nil {
		//now, overwrite buf[offset1:offset2] with data
		i := offset1
		var j int64
		for i < offset2 {
			buf[i] = data[j]
			i++
			j++
		}
		//and write
		n, err = d.fd.(*os.File).WriteAt(buf, begin)
		if err != nil {
			msg := "%s: AlignedWrite %d bytes (%d actual) at %d (%d actual) - err %s"
			log.Logf(msg, d.dev[len(d.dev)-3:], size, len(buf), at, begin, err)
		}
	}
	if err == nil && n < len(buf) {
		err = io.ErrShortWrite
	}

	return err
}

func (d *Device) String() string {
	return fmt.Sprintf("dev=%s ss=%d ds=%d at=%s al=%x am[%d]", d.dev, d.sectorSize, d.devSize, d.arrayType, d.alignment, len(d.arrayMetadata))
}

type Devices []*Device

func (ds Devices) String() string {
	var strs []string
	for _, d := range ds {
		strs = append(strs, d.String())
	}
	return strings.Join(strs, "\n")
}
