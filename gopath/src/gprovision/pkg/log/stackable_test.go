// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package log

import (
	"encoding/json"
	"gprovision/pkg/log/flags"
	"testing"
	"time"
)

// Test helper function returning logStack. Only suitable for testing - ignores
// logStackMtx.
func Stack() StackableLogger { return logStack }

func TestMarshalEntry(t *testing.T) {
	T, _ := time.Parse("2006", "1999")
	e := LogEntry{
		Time:  T,
		Flags: flags.EndUser | flags.Fatal | flags.Flag(0x90),
		Msg:   "test",
	}
	j, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"t":"1999-01-01T00:00:00Z","Msg":"test","Flags":"user|fatal|0x90"}`
	if string(j) != want {
		t.Errorf("marshal:\nwant %s\n got %s", want, string(j))
	}
}
