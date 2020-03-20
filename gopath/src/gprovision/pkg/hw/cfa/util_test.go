// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package cfa

import (
	"bytes"
	"gprovision/pkg/log/testlog"
	"sync"
	"testing"
	"time"
)

//func fit(msg string, rect Coord) []string
func TestFit(t *testing.T) {
	short := LcdTxt("short message")
	long := LcdTxt("This is a very long message that seemingly never ever ends")
	testdata := []struct {
		name string
		msg  LcdTxt
		rect Coord
		want []LcdTxt
	}{
		{"short", short, Coord{20, 2}, []LcdTxt{short}},
		{"short_narrow_nowrap", short, Coord{13, 2}, []LcdTxt{short}},
		{"short_narrow_wrap", short, Coord{12, 2}, Strs2LTxt("short", "message")},
		{"short_too_narrow", short, Coord{3, 2}, Strs2LTxt("sho", "rt", "mes")},
		{
			"long",
			long,
			Coord{20, 2},
			Strs2LTxt(
				"This is a very long",
				"message that",
				"seemingly never ever",
			),
		},
		{
			"long2",
			long,
			Coord{9, 255},
			Strs2LTxt(
				"This is a",
				"very long",
				"message",
				"that",
				"seemingly",
				"never",
				"ever ends",
			),
		},
		{
			"nospaces",
			bytes.Replace(long, []byte(" "), []byte("_"), -1),
			Coord{20, 255},
			Strs2LTxt(
				"This_is_a_very_long_",
				"message_that_seeming",
				"ly_never_ever_ends",
			),
		},
	}
	for _, td := range testdata {
		t.Run(td.name, func(t *testing.T) {
			got := fit(td.msg, td.rect)
			if len(got) != len(td.want) {
				t.Errorf("different number of lines: want %d, got %d", len(td.want), len(got))
				t.Logf("\nwant %#v\ngot  %#v", td.want, got)
			} else {
				for i, line := range td.want {
					if len(got[i]) > int(td.rect.Col) {
						t.Errorf("line %d: too long", i)
					}
					if !bytes.Equal(line, got[i]) {
						t.Errorf("line %d:\nwant %s\ngot  %s", i, line, got[i])
					}
				}
			}
		})
	}
}

//func wrapPos(msg string, width byte) byte
func TestWrapPos(t *testing.T) {
	twenty := LcdTxt("01234567890123456789")
	gap := LcdTxt("012345678 0123456789")
	var hundred, threehundred LcdTxt
	for i := 0; i < 5; i++ {
		hundred = append(hundred, twenty...)
	}
	for i := 0; i < 3; i++ {
		threehundred = append(threehundred, hundred...)
	}

	testdata := []struct {
		msg           LcdTxt
		width, result byte
	}{
		{twenty, 5, 0},
		{twenty, 50, 20},
		{gap, 50, 20},
		{twenty, 8, 0},
		{twenty, 9, 0},
		{gap, 8, 0},
		{gap, 9, 9},
		{gap, 10, 9},
		{threehundred[:255], 255, 255},
		{threehundred, 255, 0},
	}
	for i, td := range testdata {
		res := wrapPos(td.msg, td.width)
		if res != td.result {
			t.Errorf("%d: want %d got %d", i, td.result, res)
		}
	}
}

func TestTickDistrib(t *testing.T) {
	tlog := testlog.NewTestLog(t, true, false)
	in := make(chan time.Time, 1)
	tm := NewTickDistrib(in, 5)
	var locks [5]sync.Mutex //make race detector happy
	var counts [5]int
	var outs [5]<-chan time.Time
	for i := 0; i < 5; i++ {
		outs[i] = tm.Get(uint(i))
		go func(idx int) {
			for {
				_, ok := <-outs[idx]
				if !ok {
					tlog.Logf("closed: %d %#v", idx, outs[idx])
					break
				}
				locks[idx].Lock()
				counts[idx]++
				locks[idx].Unlock()
				tlog.Logf("%*scounts[%d]++: %d", idx, "", idx, counts[idx])
			}
		}(i)
	}
	now := time.Now()
	in <- now
	time.Sleep(time.Millisecond * 10)
	for i := 0; i < 5; i++ {
		locks[i].Lock()
		if counts[i] != 1 {
			t.Errorf("%d: want 1 got %d", i, counts[i])
		}
		locks[i].Unlock()
	}
	in <- now
	time.Sleep(time.Millisecond * 10)
	in <- now
	time.Sleep(time.Millisecond * 10)
	in <- now
	time.Sleep(time.Millisecond * 10)
	in <- now
	time.Sleep(time.Millisecond * 10)
	for i := 0; i < 5; i++ {
		locks[i].Lock()
		if counts[i] != 5 {
			t.Errorf("%d: want 5 got %d", i, counts[i])
		}
		locks[i].Unlock()
	}
	tm.Stop()
	time.Sleep(time.Millisecond * 10) //gives goroutines time to shut down and log anything left
	if t.Failed() {
		t.Log(tlog.Buf.String())
	}
}

//func visible(txt LcdTxt, start, width byte, ellipsis bool) LcdTxt
func TestVisible(t *testing.T) {
	testdata := []struct {
		name         string
		in, want     LcdTxt
		start, width byte
		ellipsis     bool
	}{
		{
			name:  "fits",
			in:    LcdTxt("short"),
			want:  LcdTxt("short"),
			width: 20,
		},
		{
			name:  "long no ellipsis",
			in:    LcdTxt("long no ellipsis no fit"),
			want:  LcdTxt("long no ellipsis no "),
			width: 20,
		},
		{
			name:  "long no ellipsis 2",
			in:    LcdTxt("long no ellipsis no fit"),
			want:  LcdTxt("no ellipsis no fit"),
			start: 5,
			width: 20,
		},
		{
			name:     "long ellipsis",
			in:       LcdTxt("long ellipsis does not fit"),
			want:     LcdTxt("long ellipsis does \x15"),
			width:    20,
			ellipsis: true,
		},
		{
			name:     "long ellipsis 2",
			in:       LcdTxt("long ellipsis does not fit"),
			want:     LcdTxt("\x14g ellipsis does no\x15"),
			start:    2,
			width:    20,
			ellipsis: true,
		},
		{
			name:     "long ellipsis 3",
			in:       LcdTxt("long ellipsis does not fit"),
			want:     LcdTxt("\x14llipsis does not f\x15"),
			start:    5,
			width:    20,
			ellipsis: true,
		},
		{
			name:     "long ellipsis 4",
			in:       LcdTxt("long ellipsis does not fit"),
			want:     LcdTxt("\x14lipsis does not fit"),
			start:    6,
			width:    20,
			ellipsis: true,
		},
		{
			name:     "long ellipsis 5",
			in:       LcdTxt("long ellipsis does not fit"),
			want:     LcdTxt("\x14ipsis does not fit"),
			start:    7,
			width:    20,
			ellipsis: true,
		},
		{
			name:  "1",
			in:    LcdTxt("s"),
			want:  LcdTxt("s"),
			width: 20,
		},
		{
			name:     "1e",
			in:       LcdTxt("s"),
			want:     LcdTxt("s"),
			width:    20,
			ellipsis: true,
		},
		{
			name: "0",
			in:   LcdTxt("s"),
		},
		{
			name:     "0e",
			in:       LcdTxt("s"),
			ellipsis: true,
		},
		{
			name:     "4e",
			in:       LcdTxt("shor"),
			want:     LcdTxt("ho"),
			start:    1,
			width:    2,
			ellipsis: true,
		},
		{
			name:     "o",
			in:       LcdTxt("s"),
			start:    1,
			width:    18,
			ellipsis: true,
		},
	}
	for _, td := range testdata {
		t.Run(td.name, func(t *testing.T) {
			tlog := testlog.NewTestLog(t, true, false)
			got := visible(td.in, td.start, td.width, td.ellipsis)
			if !bytes.Equal(got, td.want) {
				t.Errorf("\n got %q (%d)\nwant %q (%d)", got, len(got), td.want, len(td.want))
				t.Log(tlog.Buf.String())
			}
		})
	}
}
