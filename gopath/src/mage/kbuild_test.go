// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

// +build mage

package main

import (
	"bytes"
	"testing"
)

//kernel config fragments
var (
	kcfg = []byte(`#
# Automatically generated file; DO NOT EDIT.
# Linux/x86 4.19.16 Kernel Configuration
#

#
# Compiler: gcc (Gentoo 7.3.0-r3 p1.4) 7.3.0
#
CONFIG_CC_IS_GCC=y
CONFIG_GCC_VERSION=70300
CONFIG_CLANG_VERSION=0
CONFIG_IRQ_WORK=y
CONFIG_BUILDTIME_EXTABLE_SORT=y
CONFIG_THREAD_INFO_IN_TASK=y

#
# General setup
#
CONFIG_INIT_ENV_ARG_LIMIT=32
# CONFIG_COMPILE_TEST is not set
CONFIG_LOCALVERSION="-mfg.pxe"
# CONFIG_LOCALVERSION_AUTO is not set
CONFIG_INITRAMFS_SOURCE="/path/to/old/initramfs.cpio"
# CONFIG_RD_GZIP is not set
`)
	kcfgOut = []byte(`#
# Automatically generated file; DO NOT EDIT.
# Linux/x86 4.19.16 Kernel Configuration
#

#
# Compiler: gcc (Gentoo 7.3.0-r3 p1.4) 7.3.0
#
CONFIG_CC_IS_GCC=y
CONFIG_GCC_VERSION=70300
CONFIG_CLANG_VERSION=0
CONFIG_IRQ_WORK=y
CONFIG_BUILDTIME_EXTABLE_SORT=y
CONFIG_THREAD_INFO_IN_TASK=y

#
# General setup
#
CONFIG_INIT_ENV_ARG_LIMIT=32
# CONFIG_COMPILE_TEST is not set
CONFIG_LOCALVERSION="-somelocalversion"
# CONFIG_LOCALVERSION_AUTO is not set
CONFIG_INITRAMFS_SOURCE="/my/path/to/initramfs.cpio"
# CONFIG_RD_GZIP is not set
`)
	kcfgUnset = []byte(`#
# Automatically generated file; DO NOT EDIT.
# Linux/x86 4.19.16 Kernel Configuration
#

#
# Compiler: gcc (Gentoo 7.3.0-r3 p1.4) 7.3.0
#
CONFIG_CC_IS_GCC=y
CONFIG_GCC_VERSION=70300
CONFIG_CLANG_VERSION=0
CONFIG_IRQ_WORK=y
CONFIG_BUILDTIME_EXTABLE_SORT=y
CONFIG_THREAD_INFO_IN_TASK=y

#
# General setup
#
CONFIG_INIT_ENV_ARG_LIMIT=32
# CONFIG_COMPILE_TEST is not set
# CONFIG_LOCALVERSION is not set
# CONFIG_LOCALVERSION_AUTO is not set
CONFIG_INITRAMFS_SOURCE="/path/to/old/initramfs.cpio"
# CONFIG_RD_GZIP is not set
`)
)

//func kOptSet(cfg []byte, key, val string) ([]byte, error)
func TestKOptSet(t *testing.T) {
	out1, err := kOptSet(kcfg, "CONFIG_LOCALVERSION", `"-somelocalversion"`, true)
	if err != nil {
		t.Error(err)
		return
	}
	out2, err := kOptSet(out1, "CONFIG_INITRAMFS_SOURCE", "/my/path/to/initramfs.cpio", true)
	if err != nil {
		t.Error(err)
		return
	}
	if !bytes.Equal(out2, kcfgOut) {
		t.Errorf("mismatch - want\n%s\n---- got ----\n%s\n", kcfgOut, out2)
		return
	}
	out3, err := kOptSet(kcfg, "CONFIG_SLAB_FREELIST_RANDOM", "some value", false)
	if err == nil {
		t.Errorf("should fail to set missing option, but did not. output:\n%s", out3)
		return
	}

	out4, err := kOptSet(kcfg, "ONFIG_LOCALVERSION", "-somelocalversion", false)
	if err == nil {
		t.Errorf("given key should not match but it did. out:\n%s", out4)
		return
	}
}

//func kOptUnset(cfg []byte, key string) ([]byte, error)
func TestKOptUnset(t *testing.T) {
	out, err := kOptUnset(kcfg, "CONFIG_LOCALVERSION")
	if err != nil {
		t.Error(err)
	}
	if !bytes.Equal(out, kcfgUnset) {
		t.Errorf("mismatch - want\n%s\n---- got ----\n%s\n", kcfgUnset, out)
	}
	out, err = kOptUnset(kcfg, "ONFIG_LOCALVERSION")
	if err == nil {
		t.Errorf("should have failed to unset missing option\noutput=\n%s", out)
	}
}
