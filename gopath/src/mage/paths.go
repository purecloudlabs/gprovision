// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

// +build mage

package main

import (
	"gprovision/testing/util"
	fp "path/filepath"
	"strings"

	"github.com/magefile/mage/sh"
)

//paths shared by jobs, as well as path-related utilty functions

//vars the user may wish to modify. override in another file.
var (
	// If you have proprietary cmds, set this variable and put the code in
	// subdirs (img,windows,util). Those commands will be used in the same
	// manner as non-proprietary ones.
	//
	// To keep things clean, recommend initializing in an init() function in a
	// separate file.
	ProprietaryCmdDir string

	// Set to the prefix used for accessing blobstore, e.g. s3://bucketname/blah/
	// Support for other blob stores is not yet implemented.
	//
	// As with ProprietaryCmdDir, recommend setting this in an init() function
	// in a separate file.
	BlobstorePrefix string

	// "normal" kernel with factory restore functionality
	KBootName = "norm_boot"
	// provisioning kernel with support for initial imaging, QA, etc
	MfgName = "provision.pxe"
)

//other vars the user is less likely to want to modify
var (
	RepoRoot, WorkDir, ArtifactDir string

	//paths for compiled kernel images
	KNoInitramfs, KBoot, KMfg string

	//kernel build dir
	KBuild string

	//init binary - normal, mfg
	InitBin, MfgBin string

	//go binaries that end up inside os image
	ImgAppsTxz string
	ImgCmds    []string //pattern(s) suitable for 'go list'

	//like ImgCmds, but these do not end up in image
	UtilCmds, WinCmds []string

	//qemu
	QDir, BiosDir, OvmfDir, QemuSys, QemuImg string

	//buildroot
	BRxz, BRcpio string

	//initramfs
	InitramfsBoot, InitramfsMfg string

	//fake update file for lifecycle tests
	FakeUpdate string

	//path for local copy of linter
	LinterPath string

	// paths in blobstore
	BlobstoreKDir, BlobstoreBRCombined, BlobstoreQemu, BlobstoreLinter string
)

const (
	CmdDir   = "gprovision/cmd"
	KOrgKDir = "https://cdn.kernel.org/pub/linux/kernel/v4.x/"
)

func paths() {
	var err error
	RepoRoot, err = util.RepoRoot()
	if err != nil {
		panic(err)
	}
	WorkDir = fp.Join(RepoRoot, "work")
	KBuild = fp.Join(WorkDir, "kbuild")
	ArtifactDir = fp.Join(WorkDir, "artifacts")

	KNoInitramfs = fp.Join(WorkDir, "test_kernel")
	KBoot = fp.Join(WorkDir, KBootName)
	KMfg = fp.Join(WorkDir, MfgName)

	InitBin = fp.Join(WorkDir, "init")
	MfgBin = fp.Join(WorkDir, "mfgInit")
	ImgAppsTxz = fp.Join(WorkDir, "img_apps.txz")

	ImgCmds = []string{CmdDir + "/img/..."}
	UtilCmds = []string{CmdDir + "/util/..."}
	WinCmds = []string{CmdDir + "/windows/..."}

	if len(ProprietaryCmdDir) > 0 {
		ImgCmds = append(ImgCmds, ProprietaryCmdDir+"/img/...")
		UtilCmds = append(UtilCmds, ProprietaryCmdDir+"/util/...")
		WinCmds = append(WinCmds, ProprietaryCmdDir+"/windows/...")
	}

	QDir = fp.Join(WorkDir, "qemu")
	QemuSys = fp.Join(QDir, "qemu-system-x86_64")
	QemuImg = fp.Join(QDir, "qemu-img")
	BiosDir = fp.Join(QDir, "pc-bios")
	OvmfDir = fp.Join(QDir, "ovmf")

	BRxz = fp.Join(RepoRoot, "combined.cpio.xz")
	BRcpio = fp.Join(WorkDir, "combined.cpio")

	InitramfsBoot = fp.Join(WorkDir, KBootName+".cpio")
	InitramfsMfg = fp.Join(WorkDir, "mfg.cpio")

	FakeUpdate = fp.Join(WorkDir, "fake.upd")

	LinterPath = fp.Join(WorkDir, "golangci-lint")

	BlobstoreKDir = BlobstorePrefix + "kernel-src/"
	BlobstoreBRCombined = BlobstorePrefix + "buildroot/combined.cpio.xz"
	BlobstoreQemu = BlobstorePrefix + "qemu.txz"
	BlobstoreLinter = BlobstorePrefix + "golangci-lint"
}

//expands pattern via go list - note that pattern isn't a shell glob
func pkglist(patterns ...string) ([]string, error) {
	args := []string{"list"}
	args = append(args, patterns...)
	out, err := sh.Output("go", args...)
	if err != nil {
		return nil, err
	}
	return strings.Split(out, "\n"), nil
}
