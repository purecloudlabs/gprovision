// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package testlog

import (
	"fmt"
	"gprovision/pkg/log"
	"testing"
)

//func FilterLog() LogFilter
func TestFilterLog(t *testing.T) {
	tlog := NewTestLog(t, true, false)
	log.Logf("test log")
	log.Msgf("test msg")
	tlog.Freeze()
	filtered := tlog.Filter(FilterLog())
	if len(filtered) != 1 {
		t.Errorf("need 1 got %#v", filtered)
	}

	if filtered[0] != "LOG:test log" {
		t.Errorf("mismatch %#v", filtered)
	}
}

//func FilterLogPfx(pfx string) LogFilter
func TestFilterLogPfx(t *testing.T) {
	tlog := NewTestLog(t, true, false)
	log.Logf("test log")
	log.Msgf("test msg")
	log.Logf("2 test log")
	tlog.Freeze()
	filtered := tlog.Filter(FilterLogPfx("test"))
	if len(filtered) != 1 {
		t.Errorf("need 1 got %#v", filtered)
	}

	if filtered[0] != "LOG:test log" {
		t.Errorf("mismatch %#v", filtered)
	}
	if tlog.Buf.Len() != 0 {
		t.Error("buffer must be empty")
	}
}

func ExampleFilterLogPfx() {
	//hack - necessary since example funcs take no args. always use the t passed in.
	t := &testing.T{}
	tlog := NewTestLog(t, true, false)
	log.Logf("test log")
	log.Msgf("test msg")
	log.Logf("2 test log")
	log.Logf("test log 2")
	tlog.Freeze()
	filtered := tlog.Filter(FilterLogPfx("test"))
	fmt.Println(filtered[0])
	//output: LOG:test log
}
