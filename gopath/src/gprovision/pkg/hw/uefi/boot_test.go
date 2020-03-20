// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package uefi

import (
	"testing"
)

//func OursPresent(entries BootEntryVars)bool
func TestOursPresent(t *testing.T) {
	entries := AllBootEntryVars()
	h := entries.OursPresent()
	if h {
		t.Error("expect false")
	}
	old := efiVarDir
	efiVarDir = "testdata/sys_firmware_efi_vars_2"
	entries = AllBootEntryVars()
	h = entries.OursPresent()
	if !h {
		t.Error("expect true")
	}
	efiVarDir = old
}
