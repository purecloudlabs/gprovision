// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package testlog

import (
	"gprovision/pkg/log"
	"testing"
)

//func (tlog *TstLog) LinesMustMatch(lf LogFilter, want []string)
func TestLinesMustMatch(t *testing.T) {
	tlog := NewTestLog(t, true, false)
	log.Logf("test log")
	log.Msgf("test msg")
	log.Logf("2 test log")
	tlog.Freeze()
	b := tlog.Buf.Bytes() //save for reuse
	tlog.LinesMustMatch(FilterLog(), []string{"LOG:test log", "LOG:2 test log"})
	if tlog.Buf.Len() != 0 {
		t.Error("buffer must be empty")
	}
	tlog.Buf.Write(b)
	tlog.LinesMustMatch(FilterLogPfx("test"), []string{"LOG:test log"})
	if tlog.Buf.Len() != 0 {
		t.Error("buffer must be empty")
	}
}
