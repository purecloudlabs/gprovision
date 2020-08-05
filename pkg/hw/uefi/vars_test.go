// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package uefi

import (
	"testing"

	"github.com/purecloudlabs/gprovision/pkg/log/testlog"
)

func init() {
	efiVarDir = "testdata/sys_firmware_efi_vars"
}

//func ReadBootVar(num uint16) (b BootVar)
func TestReadBootVar(t *testing.T) {
	var n uint16
	for n = 0; n < 11; n++ {
		tlog := testlog.NewTestLogNoBG(t)
		b := ReadBootVar(n)
		t.Logf(b.String())
		tlog.Freeze()
	}
}

//func AllBootEntryVars() (list []BootEntryVar)
func TestAllBootEntryVars(t *testing.T) {
	tlog := testlog.NewTestLogNoBG(t)
	bevs := AllBootEntryVars()
	tlog.Freeze()
	if len(bevs) != 11 {
		for i, e := range bevs {
			t.Logf("#%d: %s", i, e)
		}
		t.Errorf("expected 11 boot vars, got %d", len(bevs))
	}
}

//func ReadCurrentBootVar() (b *BootEntryVar)
func TestReadCurrentBootVar(t *testing.T) {
	tlog := testlog.NewTestLogNoBG(t)
	v := ReadCurrentBootVar()
	t.Log(v)
	tlog.Freeze()
	if v == nil {
		t.Fatal("nil")
	}
	if v.Number != 10 {
		t.Errorf("expected 10, got %d", v.Number)
	}
}
