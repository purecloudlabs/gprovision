// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package cfa

import (
	"gprovision/pkg/log/testlog"
	"sync"
	"testing"
	"time"
)

//func (l *Lcd) Menu(items []LcdTxt, timeout time.Duration, keyPolling bool) Choice
//uses golden files
func TestMenu(t *testing.T) {
	testdata := []struct {
		name    string
		model   Model
		items   []LcdTxt
		want    Choice
		keys    mockKeySequence
		ticks   int
		timeout time.Duration
	}{
		{
			name:    "all fit",
			model:   Cfa635,
			items:   Strs2LTxt("one", "two", "thr33", "4"),
			want:    CHOICE_NONE,
			ticks:   30,
			timeout: 10 * time.Millisecond,
		},
		{
			name:    "fit horiz",
			model:   Cfa631,
			items:   Strs2LTxt("one", "two", "thr33", "4"),
			want:    CHOICE_NONE,
			ticks:   30,
			timeout: 10 * time.Millisecond,
		},
		{
			name:    "one long",
			model:   Cfa631,
			items:   Strs2LTxt("one", "A bcdefghi jklmno pqrstu vwx yz.", "two", "thr33"),
			want:    CHOICE_NONE,
			ticks:   30,
			timeout: 20 * time.Millisecond,
		},
		{
			name:    "one long offscreen",
			model:   Cfa631,
			items:   Strs2LTxt("one", "two", "thr33", "A bcdefghi jklmno pqrstu vwx yz."),
			want:    CHOICE_NONE,
			ticks:   30,
			timeout: 10 * time.Millisecond,
		},
		{
			name:    "offscreen",
			model:   Cfa631,
			items:   Strs2LTxt("one", "two", "thr33", "4", "5", "6"),
			want:    4,
			ticks:   30,
			timeout: 15 * time.Millisecond,
			keys: mockKeySequence{
				{key: KEY_DOWN_RELEASE,
					repeat: 3,
				},
				{key: KEY_ENTER_RELEASE},
			},
		},
		{
			name:    "offscreen blank",
			model:   Cfa631,
			items:   Strs2LTxt("one", "two", "thr33", "4", "", "5", "6"),
			want:    5,
			ticks:   30,
			timeout: 15 * time.Millisecond,
			keys: mockKeySequence{
				{key: KEY_DOWN_RELEASE,
					repeat: 4,
				},
				{key: KEY_ENTER_RELEASE},
			},
		},
	}
	for _, td := range testdata {
		t.Run(td.name, func(t *testing.T) {
			wg := new(sync.WaitGroup)
			tlog := testlog.NewTestLog(t, true, false)
			lcd, err := connectTo(Mock(td.model, wg, true))
			if err != nil {
				t.Error(err)
				return
			}
			lcd.dev.MinPktInterval = time.Microsecond
			mkg := NewMockKeygen(lcd, tlog, td.keys, td.ticks /*td.syncHeadroom*/, 0)
			mkg.Drainable(mkg.SyncTick)
			tdist := NewTickDistrib(mkg.SyncTick, 2)
			mkg.Run(td.timeout)
			mkg.wg.Add(1)

			updateTicker := NewTickerFromChan(tdist.Get(0))
			scrollTicker := NewTickerFromChan(tdist.Get(1))

			go func() {
				got := lcd.menuWithTicks(td.items, mkg.done, updateTicker, scrollTicker, false, mkg.SyncTick)
				if got != td.want {
					t.Errorf("got answer %d, want %d", got, td.want)
				}
				mkg.wg.Done()
				tdist.Stop()
			}()
			mkg.wg.Wait()
			tlog.MustMatchGoldenCleaned(testlog.FilterLogPfx("Write"), decode)
			if t.Failed() {
				t.Log(tlog.Buf.String())
			}
			lcd.Close()
			wg.Wait()
		})
	}
}

//func (m *menu) draw()
//uses golden files
func TestDraw(t *testing.T) {
	testdata := []struct {
		name     string
		model    Model
		legend   Legend
		items    []LcdTxt
		activity mockKeySequence
	}{
		{
			name:   "631longoff",
			model:  Cfa631,
			legend: LegendUVDX,
			items:  Strs2LTxt("a", "aa", "loooooooong item                       ."),
			activity: mockKeySequence{
				{},
				{key: KEY_LL_RELEASE},
				{key: KEY_LL_RELEASE},
				{key: KEY_LL_RELEASE},
				{},
				{key: KEY_UL_RELEASE},
				{}, {}, {}, {}, {}, {}, {}, {}, {},
				{}, {}, {}, {}, {}, {}, {}, {}, {},
				{}, {}, {}, {}, {}, {}, {}, {}, {},
			},
		},
		{
			name:   "635long",
			model:  Cfa635,
			legend: LegendLVRX,
			items:  Strs2LTxt("a", "aa", "loooooooong item                       ."),
			activity: mockKeySequence{
				{},
				{},
				{},
			},
		},
		{
			name:  "offscreen",
			model: Cfa631,
			items: Strs2LTxt("one", "two", "thr33", "4", "5", "6"),
			activity: mockKeySequence{
				{key: KEY_DOWN_RELEASE},
				{key: KEY_DOWN_RELEASE},
				{key: KEY_DOWN_RELEASE},
				{key: KEY_DOWN_RELEASE},
				{},
				{key: KEY_ENTER_RELEASE},
				{},
			},
		},
		{
			name:  "offscreen blank",
			model: Cfa631,
			items: Strs2LTxt("one", "two", "thr33", "4", "", "5", "6"),
			activity: mockKeySequence{
				{key: KEY_DOWN_RELEASE},
				{key: KEY_DOWN_RELEASE},
				{key: KEY_DOWN_RELEASE},
				{key: KEY_DOWN_RELEASE},
				{},
				{key: KEY_DOWN_RELEASE},
				{},
				{key: KEY_ENTER_RELEASE},
				{},
			},
		},
		{
			name:  "offscreen blank ellipsis",
			model: Cfa631,
			items: Strs2LTxt("one", "two", "thr33", "4 long enough to need ellipsis", "", "5", "6"),
			activity: mockKeySequence{
				{key: KEY_DOWN_RELEASE},
				{key: KEY_DOWN_RELEASE},
				{key: KEY_DOWN_RELEASE},
				{key: KEY_DOWN_RELEASE},
				{},
				{key: KEY_DOWN_RELEASE},
				{},
				{key: KEY_ENTER_RELEASE},
				{},
			},
		},
	}

	for _, td := range testdata {
		t.Run(td.name, func(t *testing.T) {
			wg := new(sync.WaitGroup)
			tlog := testlog.NewTestLog(t, true, false)
			l, err := connectTo(Mock(td.model, wg, true))
			if err != nil {
				t.Error(err)
				return
			}
			l.dev.MinPktInterval = time.Microsecond
			if td.legend == LegendNone {
				td.legend = Legend_VDX
			}
			l.setLegend(td.legend, true)

			update := make(chan time.Time, 1)
			scroll := make(chan time.Time, 1)
			done := make(chan struct{})
			close(done)
			m := l.createMenu(td.items, done, update, scroll, false)
			for _, i := range m.items {
				i.debug(true)
			}
			now := time.Now()
			for i, a := range td.activity {
				tlog.Logf("Write seq %d begins -------------", i) //separator in golden file
				scroll <- now
				m.draw()
				m.v.update(a.key)
			}
			l.Close()
			wg.Wait()
			tlog.MustMatchGoldenCleaned(testlog.FilterLogPfx("Write"), decode)
		})
	}
}

//func (v *view) update(k KeyActivity)
func TestUpdate(t *testing.T) {
	testdata := []struct {
		name  string
		model Model
		keys  mockKeySequence
		max   int
	}{
		{
			name:  "631",
			model: Cfa631,
			keys: mockKeySequence{
				{
					key:    KEY_DOWN_RELEASE,
					repeat: 10,
				},
				{key: KEY_ENTER_RELEASE},
			},
			max: 10,
		},
		{
			name:  "635",
			model: Cfa635,
			keys: mockKeySequence{
				{
					key:    KEY_DOWN_RELEASE,
					repeat: 10,
				},
				{key: KEY_ENTER_RELEASE},
			},
			max: 10,
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
			v := view{
				l:      l,
				choice: CHOICE_NONE,
				max:    td.max,
				height: int(l.dims.Row),
			}
			//setting legend here is not strictly necessary but makes output easier to read
			l.setLegend(Legend_VDX, true)
			keyidx := 0
			for keyidx < len(td.keys) {
				tlog.Logf("Write seq %d:%d begins -------------", keyidx, td.keys[keyidx].repeat)
				if v.redrawCursor {
					_, _, err := l.sendCmd(Cmd_SetCursorPos, []byte{0, byte(v.selected - v.first)})
					if err != nil {
						t.Error(err)
					}
				}
				v.update(td.keys[keyidx].key)
				if td.keys[keyidx].repeat == 0 {
					keyidx++
				} else {
					td.keys[keyidx].repeat--
				}
			}
			l.Close()
			wg.Wait()
			tlog.Freeze()
			tlog.MustMatchGoldenCleaned(testlog.FilterLogPfx("Write"), decode)
		})
	}
}
