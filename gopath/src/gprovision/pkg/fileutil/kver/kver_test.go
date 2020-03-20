// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package kver

import (
	"bytes"
	"gprovision/pkg/log/testlog"
	"io"
	"math/rand"
	"testing"
	"time"
)

const (
	normboot = "4.19.16-norm_boot (user@host) #300 SMP Fri Jan 25 16:32:19 UTC 2019"
	ancient  = "2.6.24.111 #606 Mon Apr 14 00:06:11 CEST 2014"
)

//func GetKDesc(k io.ReadSeeker) (string, error)
func TestGetKDesc(t *testing.T) {
	items := []bufItem{
		{510, []byte{0x55, 0xaa}}, //boot sig
		{514, []byte("HdrS")},     //kernel header
		{526, []byte{0x58, 0x30}}, //add 0x200 for offset of null-terminated string
		{12870, []byte("string starting.. " + normboot + "\000 end of str")},
	}
	f, err := sparseBuf(items)
	if err != nil {
		t.Fatal(err)
	}
	str, err := GetKDesc(f)
	if err != nil {
		t.Error(err)
	}
	if str != normboot {
		t.Errorf("want %s\n got %s", normboot, str)
	}
}

type bufItem struct {
	off  int
	data []byte
}

//return buffer filled with random data, except for listed items
func sparseBuf(items []bufItem) (io.ReadSeeker, error) {
	//figure out where last byte will fall
	var last int
	for _, i := range items {
		if len(i.data)+i.off > last {
			last = len(i.data) + i.off
		}
	}
	//make buffer a bit oversize
	buf := make([]byte, last+64)
	//write random data
	rand.Read(buf)
	//then write items
	for _, i := range items {
		copy(buf[i.off:], i.data)
	}
	return bytes.NewReader(buf), nil
}

//func ParseDesc(desc string) KInfo
func TestParseDesc(t *testing.T) {
	tmust := func(tm time.Time, err error) time.Time {
		if err != nil {
			t.Error(err)
		}
		return tm
	}
	testdata := []struct {
		name, str string
		want      KInfo
	}{
		{
			name: "normboot",
			str:  normboot,
			want: KInfo{
				Release:   "4.19.16-norm_boot",
				Version:   "#300 SMP Fri Jan 25 16:32:19 UTC 2019",
				Builder:   "user@host",
				BuildNum:  300,
				BuildTime: tmust(time.Parse(time.RFC3339, "2019-01-25T16:32:19Z")), //equivalent
				Maj:       4,
				Min:       19,
				Patch:     16,
				LocalVer:  "norm_boot",
			},
		},
		{
			name: "ancient",
			str:  ancient,
			want: KInfo{
				Release:   "2.6.24.111",
				Version:   "#606 Mon Apr 14 00:06:11 CEST 2014",
				Builder:   "",
				BuildNum:  606,
				BuildTime: tmust(time.Parse(time.RFC3339, "2014-04-14T00:06:11Z")), //equivalent
				Maj:       2,
				Min:       6,
				Patch:     24,
				LocalVer:  "",
			},
		},
	}
	for _, td := range testdata {
		t.Run(td.name, func(t *testing.T) {
			tlog := testlog.NewTestLog(t, true, false)
			defer func() {
				tlog.Freeze()
				if t.Failed() {
					t.Log(tlog.Buf.String())
				}
			}()
			ki, err := ParseDesc(td.str)
			if err != nil {
				t.Error(err)
			}
			if !ki.Equal(td.want) {
				t.Error("mismatch")
			}
			if t.Failed() {
				t.Logf("\nwant %#v\ngot  %#v", td.want, ki)
			}
		})
	}
}

func (l KInfo) Equal(r KInfo) bool {
	return l.Release == r.Release &&
		l.Builder == r.Builder &&
		l.Version == r.Version &&
		l.BuildNum == r.BuildNum &&
		l.BuildTime.Equal(r.BuildTime) &&
		l.Maj == r.Maj &&
		l.Min == r.Min &&
		l.Patch == r.Patch &&
		l.LocalVer == r.LocalVer
}
