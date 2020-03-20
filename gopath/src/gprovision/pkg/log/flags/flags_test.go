// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package flags

import (
	"testing"
)

func TestString(t *testing.T) {
	for i, td := range []struct {
		f    Flag
		want string
	}{
		{f: EndUser | Fatal, want: "user|fatal"},
		{f: EndUser, want: "user"},
		{f: NA, want: ""},
		{f: Flag(0), want: ""},
		{f: Flag(0x1), want: "user"},
		{f: Flag(0x2), want: "fatal"},
		{f: Flag(0x4), want: "not file"},
		{f: Flag(0x8), want: "not wire"},
		{f: Flag(0x1232), want: "fatal|0x1230"},
		{f: Flag(0x1234), want: "not file|0x1230"},
		{f: Flag(0x7890), want: "0x7890"},
		{f: Flag(0x7899), want: "user|not wire|0x7890"},
	} {
		s := td.f.String()
		if s != td.want {
			t.Errorf("%d 0x%x: want %s, got %s", i, int(td.f), td.want, s)
		}
	}
}
