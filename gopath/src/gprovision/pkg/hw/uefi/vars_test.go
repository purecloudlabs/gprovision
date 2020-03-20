// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package uefi

import (
	"gprovision/pkg/log/testlog"
	"testing"
)

func init() {
	efiVarDir = "testdata/sys_firmware_efi_vars"
}

//func ReadBootVar(num uint16) (b BootVar)
func TestReadBootVar(t *testing.T) {
	tlog := testlog.NewTestLog(t, true, false)
	var n uint16
	for n = 0; n < 11; n++ {
		b := ReadBootVar(n)
		tlog.Freeze()
		l := tlog.Buf.String()
		if len(l) > 0 {
			t.Logf("%s", b)
			t.Error(l)
		}
	}
}

//func AllBootEntryVars() (list []BootEntryVar)
func TestAllBootEntryVars(t *testing.T) {
	tlog := testlog.NewTestLog(t, true, false)
	bevs := AllBootEntryVars()
	tlog.Freeze()
	l := tlog.Buf.String()
	if l != "" {
		t.Error(l)
	}
	if len(bevs) != 11 {
		for i, e := range bevs {
			t.Logf("#%d: %s", i, e)
		}
		t.Errorf("expected 10 boot vars, got %d", len(bevs))
	}
}
