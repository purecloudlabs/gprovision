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
	"io/ioutil"
	"os"
	"testing"
)

type alignedReadTestData struct {
	at, reqSize  int64
	resBufSize   int
	resBufBegins []byte //bytes that should match beginning of result buffer
	expectError  bool
}

//func (d * Device) AlignedRead(at, size int64) (buf []byte, err error)
func TestAlignedRead(t *testing.T) {
	d := new(Device)
	d.fd, d.devSize = createTestFile(t)
	defer os.Remove(d.fd.(*alignedFile).f.Name())
	defer d.fd.Close()

	testData := []alignedReadTestData{
		/* at, reqSize, resBufSize, resBufBegins, expectError */
		{0, 512, 512, []byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x1}, false},
		{4, 512, 512, []byte{0x0, 0x0, 0x0, 0x1}, false},
		{32766, 512, 2, []byte{0x1f, 0xff}, false},
		{-5, 512, 5, []byte{0xfe, 0x0, 0x0, 0x1f, 0xff}, false},
		{-1024, 512, 512, []byte{0x0, 0x0, 0x1f, 0x0}, false},
		{-32769, 512, 0, nil, true},
		{32769, 512, 0, nil, true},
	}

	for _, a := range []int{512, 1024, 4096, 8192, 16384 /*,32768*/} {
		t.Logf("testing with required alignment == %d", a)
		d.fd.(*alignedFile).requiredAlignment = a
		d.alignment = 0
		for i, row := range testData {
			buf, err := d.AlignedRead(row.at, row.reqSize)
			if (err != nil) != row.expectError {
				t.Fatalf("fatal at row %d: expectError == %t, err == %s, last alignment==%d", i, row.expectError, err, d.alignment)
			}
			if err == nil && buf == nil {
				t.Fatalf("fatal at row %d: buf and err both nil", i)
			}
			if len(buf) != row.resBufSize {
				t.Errorf("error at row %d: len(buf)==%d, wanted %d", i, len(buf), row.resBufSize)
			}
			if d.alignment > int64(a) && !row.expectError {
				t.Errorf("error at row %d: alignment==%d, wanted %d", i, d.alignment, a)
			}
			l := len(row.resBufBegins)
			b := len(buf)
			if b < l {
				t.Fatalf("fatal at row %d: len(buf) == %d, < len(resBufBegins) == %d", i, b, l)
			}
			if !bytes.Equal(buf[:l], row.resBufBegins) {
				t.Errorf("error at row %d: wrong buffer contents.\n%#v !=\n%#v", i, buf[:l], row.resBufBegins)
			}
		}
	}
}

/* alignedFile implements our ReadWriteSeekCloser using
 * os.File, but overrides Seek and Read to force alignment
 */
type alignedFile struct {
	f                 *os.File
	requiredAlignment int
	log               *testing.T
}

func (d *alignedFile) Seek(at int64, whence int) (n int64, err error) {
	if (d.requiredAlignment > 0) && (at%int64(d.requiredAlignment) != 0) {
		d.log.Logf("alignedFile.Seek: rejected alignment. at==%d, whence==%d", at, whence)
		return 0, EAlignment
	}
	return d.f.Seek(at, whence)
}
func (d *alignedFile) Read(buf []byte) (int, error) {
	if (d.requiredAlignment > 0) && (len(buf)%d.requiredAlignment != 0) {
		d.log.Logf("alignedFile.Read: rejected alignment. len(buf)==%d", len(buf))
		return 0, EAlignment
	}
	return d.f.Read(buf)
}
func (d *alignedFile) Close() error                  { return d.f.Close() }
func (d *alignedFile) Fd() uintptr                   { return d.f.Fd() }
func (d *alignedFile) Write(buf []byte) (int, error) { return d.f.Write(buf) }

func createTestFile(t *testing.T) (fd *alignedFile, dsize uint64) {
	var err error
	fd = new(alignedFile)
	fd.requiredAlignment = 4096
	fd.log = t
	fd.f, err = ioutil.TempFile("", "")
	if err != nil {
		t.Fatalf("err %s", err)
		return
	}
	//fill 32kb with 32-bit numbers (8k)
	var i uint32
	var reps uint32 = 8192
	dsize = uint64(reps) * 4
	b := make([]byte, 4)
	for i = 0; i < reps; i++ {
		//var b [4]byte
		binary.BigEndian.PutUint32(b, i)
		_, err := fd.f.Write(b)
		if err != nil {
			t.Fatalf("err %s", err)
			fd = nil
			return
		}
	}
	_, err = fd.f.Seek(0, 0)
	if err != nil {
		fd = nil
		t.Fatalf("err %s", err)
	}
	return
}

func TestAlign(t *testing.T) {
	//func align(at int64, how int64) (res int64, off int64)
	testData := [][4]int64{
		{4, 8, 0, 4},
		{8, 4, 8, 0},
		{0, 2048, 0, 0},
		{512, 8192, 0, 512},
		{1000, 512, 512, 488},
		{-512, 512, -512, 0},
		{-515, 512, -1024, 509},
		{32766, 512, 32256, 510},
		{-32769, 16384, -49152, 16383},
		{32769, 16384, 32768, 1},
	}
	var r, o int64
	for i, row := range testData {
		r, o = align(row[0], row[1])
		if r != row[2] {
			t.Errorf("row %d: want res %d, got %d", i, row[2], r)
		}
		if o != row[3] {
			t.Errorf("row %d: want off %d, got %d", i, row[3], o)
		}
	}
}
