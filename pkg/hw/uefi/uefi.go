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
	fp "path/filepath"
	"strings"

	futil "github.com/purecloudlabs/gprovision/pkg/fileutil"
	"github.com/purecloudlabs/gprovision/pkg/log"
)

//return true if the system booted via UEFI (as opposed to legacy)
func BootedUEFI() bool {
	_, err := os.Stat("/sys/firmware/efi")
	return (err == nil)
}

// Check if given drive contains /efi/boot/bootx64.efi (case insensitive).
// Does not try to determine if bootx64.efi is valid, if the firmware supports
// the filesystem type, etc.
func DriveIsBootable(mountpoint string) bool {
	bootx := fp.Join("efi", "boot", "bootx64.efi")
	results, err := futil.FindCaseInsensitive(mountpoint, "bootx64.efi", 2)
	if err != nil {
		log.Logf("determining if drive is bootable: %s", err)
		return false
	}
	mountpoint = mountpoint + string(os.PathSeparator)
	for _, r := range results {
		path := strings.TrimPrefix(strings.ToLower(r), mountpoint)
		if path == bootx {
			return true
		}
	}
	return false
}
