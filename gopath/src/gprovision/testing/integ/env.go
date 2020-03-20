// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package integ

import (
	gtst "gprovision/testing"
	"os"
	"strconv"
	"strings"
)

func EnvFromSys(cpus int, kvm bool) {
	if _, ok := os.LookupEnv("UROOT_QEMU"); !ok {
		args := []string{
			"qemu-system-x86_64",
			"-smp", strconv.Itoa(cpus),
			"-cpu", "qemu64", //do not use host, as capabilities would vary by system, losing repeatability
			"-L", "/usr/share/qemu",
		}
		if kvm {
			args = append(args, "-enable-kvm")
		}
		os.Setenv("UROOT_QEMU", strings.Join(args, " "))
	}
	if _, ok := os.LookupEnv("OVMF_DIR"); !ok {
		os.Setenv("OVMF_DIR", "/usr/share/edk2-ovmf")
	}
}

func CheckEnv(t gtst.TB) (rootDir, mfgKernel string) {
	msg := `Integ tests are skipped unless certain env vars (including %s) are set.
	To run integ tests: 'mage tests:integ'`
	_, ok := os.LookupEnv("UROOT_QEMU")
	if !ok {
		t.Skipf(msg, "UROOT_QEMU")
	}
	rootDir = os.Getenv("INFRA_ROOT")
	if len(rootDir) == 0 {
		t.Skipf(msg, "INFRA_ROOT")
	}
	mfgKernel = os.Getenv("UROOT_KERNEL")
	if len(mfgKernel) == 0 {
		t.Skipf(msg, "UROOT_KERNEL")
	}
	return
}
