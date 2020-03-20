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

//just prints out enum values. trivial sanity check that I got
//the iota + bit shift correct when setting values
func TestKPBits(t *testing.T) {
	got := ""
	for i, b := range []KeyMask{
		KP_UL,
		KP_UR,
		KP_LL,
		KP_LR,
		KP_ALL_631,

		KP_UP,
		KP_ENTER,
		KP_EXIT,
		KP_LEFT,
		KP_RIGHT,
		KP_DOWN,
		KP_UVDX_635,
		KP_LVRX_635,
		KP_ALL_635,
	} {
		got += fmt.Sprintf("\n%02d %06b 0x%02x %02d", i, b, b, b)
	}
	want := `
00 000001 0x01 01
01 000010 0x02 02
02 000100 0x04 04
03 001000 0x08 08
04 001111 0x0f 15
05 000001 0x01 01
06 000010 0x02 02
07 000100 0x04 04
08 001000 0x08 08
09 010000 0x10 16
10 100000 0x20 32
11 100111 0x27 39
12 011110 0x1e 30
13 111111 0x3f 63`
	if got != want {
		t.Errorf("want\n%s\ngot\n%s", want, got)
	}
}

func TestSetLegend(t *testing.T) {
	testdata := []struct {
		name   string
		model  Model
		legend Legend
	}{
		{
			name:   "631lvrx",
			model:  Cfa631,
			legend: LegendLVRX,
		},
		{
			name:   "631uvdx",
			model:  Cfa631,
			legend: LegendUVDX,
		},
		{
			name:   "635lvrx",
			model:  Cfa635,
			legend: LegendLVRX,
		},
		{
			name:   "635uvdx",
			model:  Cfa635,
			legend: LegendUVDX,
		},
		{
			name:   "635_vdx",
			model:  Cfa635,
			legend: Legend_VDX,
		},
		{
			name:   "635uv_x",
			model:  Cfa635,
			legend: LegendUV_X,
		},
		{
			name:   "635none",
			model:  Cfa635,
			legend: LegendNone,
		},
		{
			name:   "631none",
			model:  Cfa631,
			legend: LegendNone,
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
			l.setLegend(td.legend, true)
			l.setLegend(LegendNone, true)
			l.Close()
			wg.Wait()
			tlog.MustMatchGoldenCleaned(testlog.FilterLogPfx("Write"), decode)
		})
	}
}
