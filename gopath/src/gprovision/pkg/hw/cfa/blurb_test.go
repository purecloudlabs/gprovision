// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package cfa

import (
	"fmt"
	"gprovision/pkg/log/testlog"
	"sync"
	"testing"
	"time"
)

//func (hs *horizScroller) updateState() (change bool)
func TestHorizScroll(t *testing.T) {
	want := []string{
		//pos,state,displayed,pos visualization
		"00 01 |Random tes| ^",
		"00 02 |Random tes| ^",
		"00 03 |Random tes| ^",
		"01 04 |andom test|  ^",
		"02 05 |ndom test |   ^",
		"03 06 |dom test t|    ^",
		"04 07 |om test te|     ^",
		"05 08 |m test tex|      ^",
		"06 09 | test text|       ^",
		"06 10 | test text|       ^",
		"06 00 | test text|       ^",
		"00 01 |Random tes| ^",
		"00 02 |Random tes| ^",
		"00 03 |Random tes| ^",
		"01 04 |andom test|  ^",
		//...
	}
	hs := &horizScroller{txt: []byte("Random test text"), availChars: 10}
	got := []string{}
	for i := 0; i < 20; i++ {
		hs.updateState()
		txt := truncate(hs.txt[hs.pos:], hs.availChars)
		got = append(got, fmt.Sprintf("%02d %02d |%s%*s| %*s", hs.pos, hs.scrollState, txt, int(hs.availChars)-len(txt), "", hs.pos+1, "^"))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d: got % 40s want %s", i, got[i], want[i])
		}
	}
	if t.Failed() {
		for _, l := range got {
			t.Log(l)
		}
	}
}

func TestVertScroller(t *testing.T) {
	testdata := []struct {
		name        string
		model       Model
		lines       []LcdTxt
		start, dims Coord
	}{
		{
			name:  "631 short fits",
			model: Cfa631,
			lines: Strs2LTxt("631 short fits"),
			dims:  Coord{Row: 1, Col: 19},
		},
		{
			name:  "631 long fits",
			model: Cfa631,
			lines: Strs2LTxt("631 long fits long a", "second line fits too"),
			dims:  Coord{Row: 1, Col: 19},
		},
		{
			name:  "631 long overflow",
			model: Cfa631,
			lines: Strs2LTxt("631 long overflow", "second line fits", "...and overflow"),
			dims:  Coord{Row: 1, Col: 19},
		},
		{
			name:  "635 over",
			model: Cfa635,
			lines: Strs2LTxt("635 long overflow", "second line fits", "third", "fourth", "...and overflow"),
			dims:  Coord{Row: 3, Col: 19},
		},
		{
			name:  "635",
			model: Cfa635,
			lines: Strs2LTxt(),
			dims:  Coord{Row: 3, Col: 19},
		},
	}
	for _, td := range testdata {
		t.Run(td.name, func(t *testing.T) {
			wg := new(sync.WaitGroup)
			tlog := testlog.NewTestLog(t, true, false)
			l, err := connectTo(Mock(td.model, wg, true))
			if err != nil {
				t.Error(err)
			}
			l.dev.MinPktInterval = time.Microsecond
			vs := NewVertScroller(l, td.lines, td.start, td.dims)
			for i := 0; i < 3; i++ {
				change, err := vs.draw(false)
				if err != nil {
					t.Error(err)
				}
				if !change {
					t.Errorf("no change")
				}
			}
			l.Close()
			wg.Wait()
			tlog.MustMatchGoldenCleaned(testlog.FilterLogPfx("Write"), decode)
		})
	}
}

func TestBlurb(t *testing.T) {
	testdata := []struct {
		name         string
		model        Model
		line         LcdTxt
		start, dims  Coord
		shouldChange []bool
		shouldErr    []bool
	}{
		{
			name:         "631 short fits",
			model:        Cfa631,
			line:         LcdTxt("631 short fits"),
			dims:         Coord{Row: 1, Col: 19},
			shouldChange: []bool{true, false, false},
		},
		{
			name:         "631 short small window",
			model:        Cfa631,
			line:         LcdTxt("631 small wndw"),
			start:        Coord{Row: 0, Col: 14},
			dims:         Coord{Row: 1, Col: 19},
			shouldChange: []bool{false, false, false},
			shouldErr:    []bool{true, true, true},
		},
		{
			name:         "631 long fits",
			model:        Cfa631,
			line:         LcdTxt("631 long fits long a second line fits too"),
			dims:         Coord{Row: 1, Col: 19},
			shouldChange: []bool{true, true, true},
		},
		{
			name:         "631 long overflow",
			model:        Cfa631,
			line:         LcdTxt("631 long overflow second line fits ...and overflow"),
			dims:         Coord{Row: 1, Col: 19},
			shouldChange: []bool{true, true, true},
		},
		{
			name:         "635",
			model:        Cfa635,
			dims:         Coord{Row: 3, Col: 19},
			shouldChange: []bool{true, false, false},
		},
		{
			name:         "635 over",
			model:        Cfa635,
			line:         LcdTxt("635 long overflow second line fits third         fourth asfdsdaf dsf ...and overflow"),
			dims:         Coord{Row: 3, Col: 19},
			shouldChange: []bool{true, true, true},
		},
	}
	for _, td := range testdata {
		t.Run(td.name, func(t *testing.T) {
			wg := new(sync.WaitGroup)
			tlog := testlog.NewTestLog(t, true, false)
			l, err := connectTo(Mock(td.model, wg, true))
			if err != nil {
				t.Error(err)
			}
			l.dev.MinPktInterval = time.Microsecond
			b := NewBlurb(l, td.line, td.start, td.dims)
			for i, shouldchange := range td.shouldChange {
				tlog.Logf("Write seq %d begins -------------", i) //separator in golden file
				var shoulderr bool
				if i < len(td.shouldErr) {
					//if array is short, shoulderr is false
					shoulderr = td.shouldErr[i]
				}
				didChange, err := b.draw(false)
				if shoulderr && err == nil {
					t.Error("should error")
				} else if !shoulderr && err != nil {
					t.Errorf("%s for\n%#v", err, b)
				}
				if shouldchange != didChange {
					t.Errorf("change err - got %t want %t", didChange, shouldchange)
				}
			}
			if t.Failed() {
				t.Logf("log:\n%s", tlog.Buf.String())
			}
			l.Close()
			wg.Wait()
			tlog.MustMatchGoldenCleaned(testlog.FilterLogPfx("Write"), decode)
		})
	}
}
