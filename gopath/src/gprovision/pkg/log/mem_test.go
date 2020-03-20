// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package log_test

// Note that this is package log_test, not log. Ensures that we expose enough
// functions to make testing possible from other packages.

import (
	"gprovision/pkg/log"
	"gprovision/pkg/log/flags"
	"testing"
	"time"
)

func TestMemLog(t *testing.T) {
	log.DefaultLogStack()
	defer log.DefaultLogStack() //cleanup when test is done
	T, err := time.Parse("2006", "1999")
	if err != nil {
		t.Fatal(err)
	}
	e := log.LogEntry{
		Time:  T,
		Msg:   "interesting event",
		Flags: flags.EndUser,
	}
	stack := log.Stack()
	stack.AddEntry(e)
	entries := log.StoredEntries()
	if len(entries) != 1 {
		t.Error("wrong entries", entries)
	}
	want := "-- 19990101_0000 -- interesting event"
	got := entries[0].String()
	if want != got {
		t.Errorf("mem:\nwant %q\ngot  %q", want, got)
	}
}
