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

func TestMock(t *testing.T) {
	wg := new(sync.WaitGroup)
	tlog := testlog.NewTestLog(t, true, false)
	lcd, err := connectTo(Mock(Cfa635, wg, false))
	if err != nil {
		t.Error(err)
		return
	}
	lcd.dev.MinPktInterval = time.Microsecond
	err = lcd.Write(Coord{0, 0}, LcdTxt("blah blah blah"))
	if err != nil {
		t.Error(err)
	}
	success, err := lcd.Ping()
	if err != nil {
		t.Error(err)
	}
	if !success {
		t.Error("ping failed")
	}
	entries := []LcdTxt{LcdTxt("one"), LcdTxt("two"), LcdTxt("three"), LcdTxt("4")}
	updateTicker := NewTicker(time.Second / 100)
	scrollTicker := NewTicker(time.Second / 100)
	done := make(chan struct{})
	go func() {
		time.Sleep(time.Second / 80)
		close(done)
	}()
	choice := lcd.menuWithTicks(entries, done, updateTicker, scrollTicker, true, nil)
	if choice != CHOICE_NONE {
		t.Errorf("wrong choice %d", choice)
	}
	lcd.Close()
	wg.Wait()
	if t.Failed() {
		t.Log(tlog.Buf.String())
	}
}

//func decodeOp(op string) operation
func TestDecodeOp(t *testing.T) {
	testdata := []struct {
		name, in string
		want     string
	}{
		{
			name: "legend",
			in:   "Write([]byte{0x20, 0x1, 0x0, 0x2f, 0xdc})=(5,<nil>)",
			want: "> Write CMD_SetKeyLegend={00}",
		},
		{
			name: "write",
			in:   "LOG:Write([]byte{0x1f, 0x12, 0x0, 0x0, 0x6f, 0x6e, 0x65, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0xcf, 0x19})=(22,<nil>)",
			want: `> Write CMD_Write, @{c00,r0} txt="one             "`,
		},
		{
			name: "vers",
			in:   "LOG:Write([]byte{0x1, 0x0, 0x9f, 0x16})=(4,<nil>)",
			want: "> Write CMD_HwFwVers",
		},
		{
			name: "stat",
			in:   "LOG:Write([]byte{0x1e, 0x0, 0xc6, 0x0})=(4,<nil>)",
			want: "> Write CMD_ReadReportingAndStatus",
		},
	}
	for _, td := range testdata {
		t.Run(td.name, func(t *testing.T) {
			op := decodeOp(td.in)
			got := op.String()
			if got != td.want {
				t.Errorf("\n got %s\nwant %s", got, td.want)
			}
		})
	}
}
