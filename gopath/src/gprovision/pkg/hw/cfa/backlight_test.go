// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package cfa

import (
	tlog "gprovision/pkg/log/testlog"
	"sync"
	"testing"
	"time"
)

//func (l *Lcd) backlightSinStep(curStep *uint, nrSteps uint)
func TestBacklightSinStep631(t *testing.T) {
	wg := new(sync.WaitGroup)
	tl := tlog.NewTestLog(t, true, false)
	l, err := connectTo(Mock(Cfa631, wg, true))
	if err != nil {
		t.Error(err)
	}
	l.dev.MinPktInterval = time.Microsecond
	var c uint = 0
	var n uint = 20
	for i := 0; i < 50; i++ {
		err = l.backlightSinStep(&c, n)
		if err != nil {
			t.Error(err)
		}
	}
	//hex values represented as strings
	expect631 := []string{
		"8", "b", "e", "11", "13",
		"16", "18", "19", "1b", "1b",
		"1c", "1b", "1b", "19", "18",
		"16", "13", "11", "e", "b",
		"8", "b", "e", "11", "13",
		"16", "18", "19", "1b", "1b",
		"1c", "1b", "1b", "19", "18",
		"16", "13", "11", "e", "b",
		"8", "b", "e", "11", "13",
		"16", "18", "19", "1b", "1b",
		"0",
	}

	l.Close()
	wg.Wait()
	//LOG:Write([]byte{0xe, 0x1, 0x8, 0x47, 0x43})=(5,<nil>)
	// only care about 1-2 bytes:  ^
	cleaner := tlog.TrimAnd(tlog.TrimToIdx(len("LOG:Write([]byte{0xe, 0x1, 0x")), tlog.TrimFromSeq(","))
	tl.LinesMustMatchCleaned(tlog.FilterLogPfx("Write([]byte{0xe,"), cleaner, expect631)
}

//func (l *Lcd) backlightSinStep(curStep *uint, nrSteps uint)
func TestBacklightSinStep635(t *testing.T) {
	wg := new(sync.WaitGroup)
	tl := tlog.NewTestLog(t, true, false)
	l, err := connectTo(Mock(Cfa635, wg, true))
	if err != nil {
		t.Error(err)
	}
	l.dev.MinPktInterval = time.Microsecond
	var c uint = 0
	var n uint = 20
	for i := 0; i < 50; i++ {
		err = l.backlightSinStep(&c, n)
		if err != nil {
			t.Error(err)
		}
	}
	//hex values represented as strings
	expect635 := []string{
		"0", "f", "1e", "2d", "3a",
		"46", "50", "59", "5f", "62",
		"64", "62", "5f", "59", "50",
		"46", "3a", "2d", "1e", "f",
		"0", "f", "1e", "2d", "3a",
		"46", "50", "59", "5f", "62",
		"64", "62", "5f", "59", "50",
		"46", "3a", "2d", "1e", "f",
		"0", "f", "1e", "2d", "3a",
		"46", "50", "59", "5f", "62",
		"0",
	}
	l.Close()
	wg.Wait()
	cleaner := tlog.TrimAnd(tlog.TrimToIdx(len("LOG:Write([]byte{0xe, 0x1, 0x")), tlog.TrimFromSeq(","))
	tl.LinesMustMatchCleaned(tlog.FilterLogPfx("Write([]byte{0xe,"), cleaner, expect635)
}
