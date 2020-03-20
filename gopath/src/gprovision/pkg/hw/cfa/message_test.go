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

//func (l *Lcd) PressAnyKey(desc string, seconds int)
func TestPressAnyKey(t *testing.T) {
	wg := new(sync.WaitGroup)
	tlog := testlog.NewTestLog(t, true, false)
	l, err := connectTo(Mock(Cfa631, wg, true))
	if err != nil {
		t.Error(err)
	}
	l.dev.MinPktInterval = time.Microsecond
	press, err := l.PressAnyKey("desc", time.Second/20, time.Second/8)
	if err != nil {
		t.Error(err)
	}
	if press {
		t.Errorf("reports key press")
	}
	l.Close()
	wg.Wait()
	tlog.MustMatchGoldenCleaned(testlog.FilterLogPfx("Write([]byte{0x"), decode)
}

func TestPressAnyKey2(t *testing.T) {
	if testing.Short() {
		t.Skip("timing sensitive, more likely to fail on ci")
	}
	wg := new(sync.WaitGroup)
	tlog := testlog.NewTestLog(t, true, false)
	l, err := connectTo(Mock(Cfa631, wg, true))
	if err != nil {
		t.Error(err)
	}
	l.dev.MinPktInterval = time.Microsecond

	//key press in future
	go func() {
		time.Sleep(time.Second / 8)
		l.dev.Events <- KEY_ENTER_PRESS
	}()
	press, err := l.PressAnyKey("desc", time.Second/20, time.Second/2)
	if err != nil {
		t.Error(err)
	}
	if !press {
		t.Errorf("reports no key press")
	}
	l.Close()
	wg.Wait()
	tlog.MustMatchGoldenCleaned(testlog.FilterLogPfx("Write([]byte{0x"), decode)
}
