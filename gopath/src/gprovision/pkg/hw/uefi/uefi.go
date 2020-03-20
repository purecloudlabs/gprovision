// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

//Package uefi manipulates UEFI boot entries and other variables. It can also
//determine whether the system booted in UEFI mode or legacy.
package uefi

import (
	"os"
)

//return true if the system booted via UEFI (as opposed to legacy)
func BootedUEFI() bool {
	_, err := os.Stat("/sys/firmware/efi")
	return (err == nil)
}
