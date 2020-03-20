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

//func (rbs *radioButtonSet) render() LcdTxt
func TestRBSRender(t *testing.T) {
	var wrapGtLt wrapStyle = [2]byte{0x3e, 0x3c} //greater than and less than - easier to visualize
	styleA := styleSet{selected: wrapArrows, deselected: wrapNone}
	styleGL := styleSet{selected: wrapGtLt, deselected: wrapNone}
	btns := []*radioButton{
		&radioButton{txt: LcdTxt("No")},
		&radioButton{txt: LcdTxt("Yes")},
	}
	testdata := []struct {
		rbs  radioButtonSet
		want LcdTxt
	}{
		{
			rbs:  radioButtonSet{btns: btns, styles: &styleA, selection: 0},
			want: LcdTxt("\x10No\x11 Yes "),
		},
		{
			rbs:  radioButtonSet{btns: btns, styles: &styleGL, selection: 1},
			want: LcdTxt(" No >Yes<"),
		},
	}
	for i, td := range testdata {
		got := td.rbs.render()
		if !bytes.Equal(got, td.want) {
			t.Errorf("%d:\nwant %q (%#v)\n got %q (%#v)", i, td.want, td.want, got, got)
		}
	}
}

//func (q *Question) Ask(timeout time.Duration) Choice
//uses golden files
func TestAskQuestion(t *testing.T) {
	testdata := []struct {
		name         string
		model        Model
		txt          LcdTxt
		opts         []LcdTxt
		want         Choice
		keys         mockKeySequence
		timeout      time.Duration
		ticks        int
		syncHeadroom int
	}{
		{
			name:         "631",
			model:        Cfa631,
			txt:          LcdTxt("Are you really certain?"),
			opts:         Strs2LTxt("No", "Yes"),
			want:         CHOICE_NONE,
			timeout:      20 * time.Millisecond,
			ticks:        6,
			syncHeadroom: 0,
			keys: mockKeySequence{
				{
					key:    KEY_NO_KEY,
					repeat: 3,
				},
				{key: KEY_DOWN_RELEASE},
			},
		},
		{
			name:         "631-2",
			model:        Cfa631,
			txt:          LcdTxt("Are you really certain?"),
			opts:         Strs2LTxt("No", "Yes"),
			want:         1,
			timeout:      20 * time.Millisecond,
			ticks:        14,
			syncHeadroom: 0,
			keys: mockKeySequence{
				{
					key:    KEY_NO_KEY,
					repeat: 3,
				},
				{key: KEY_DOWN_RELEASE},
				{key: KEY_RIGHT_RELEASE},
				{key: KEY_ENTER_RELEASE},
			},
		},
		{
			name:         "635",
			model:        Cfa635,
			txt:          LcdTxt("Are you really certain?"),
			opts:         Strs2LTxt("No", "Yes"),
			want:         CHOICE_NONE,
			timeout:      20 * time.Millisecond,
			ticks:        15,
			syncHeadroom: 1,
			keys: mockKeySequence{
				{
					key:    KEY_NO_KEY,
					repeat: 3,
				},
				{key: KEY_DOWN_RELEASE},
			},
		},
	}
	for _, td := range testdata {
		t.Run(td.name, func(t *testing.T) {
			wg := new(sync.WaitGroup)
			tlog := testlog.NewTestLog(t, true, false)
			l, err := connectTo(Mock(td.model, wg, true))
			if err != nil {
				panic(err)
			}
			l.dev.MinPktInterval = time.Microsecond
			q := &Question{
				l:   l,
				txt: td.txt,
			}
			//set up buttons, which must fit on one line
			err = q.createButtonSet(td.opts)
			if err != nil {
				t.Error(err)
			}
			mkg := NewMockKeygen(l, tlog, td.keys, td.ticks, td.syncHeadroom)
			mkg.Drainable(mkg.SyncTick)
			q.syncTick = mkg.SyncTick
			mkg.Run(td.timeout)
			mkg.wg.Add(1)
			go func() {
				ans := q.ask(mkg.done, mkg.update)
				if ans != td.want {
					t.Errorf("mismatch - want answer %d got %d", td.want, ans)
				}
				mkg.wg.Done()
			}()
			mkg.wg.Wait()
			l.Close()
			wg.Wait()
			tlog.MustMatchGoldenCleaned(testlog.FilterLogPfx("Write([]byte{0x"), decode)
		})
	}
}
