// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

// not for production use

// +build !release

package paths

import (
	"os/exec"
	fp "path/filepath"
	"strings"

	"github.com/magefile/mage/sh"

	"github.com/purecloudlabs/gprovision/pkg/log"
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
	RepoRoot, ImportPath, WorkDir, ArtifactDir string

	// GoDirs - dirs containing code; limit go test and go generate
	// to specific paths, else they will be very slow due to scanning
	// /build/brx, /work, etc - dirs with myriad files but no go.
	GoDirs []string

	//paths for compiled kernel images
	KNoInitramfs, KBoot, KMfg, KInstaller string

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
	KOrgKDir = "https://cdn.kernel.org/pub/linux/kernel/v4.x/"
)

func init() {
	var err error
	RepoRoot, err = repoRoot()
	if err != nil {
		log.Logf("Cannot determine repo root.")
	}
	WorkDir, err = workDir()
	if err != nil {
		log.Logf("Cannot determine workdir.")
	}

	cmd := exec.Command("go", "list", "-f", "{{.ImportPath}}", ".")
	cmd.Dir = RepoRoot
	out, err := cmd.Output()
	if err != nil {
		log.Logf("Cannot determine import path.")
	}
	ImportPath = strings.TrimSpace(string(out))

	KBuild = fp.Join(WorkDir, "kbuild")
	ArtifactDir = fp.Join(WorkDir, "artifacts")

	GoDirs = []string{
		ImportPath + "/cmd/...",
		ImportPath + "/pkg/...",
		ImportPath + "/proprietary/...",
		ImportPath + "/testing/...",
	}

	KNoInitramfs = fp.Join(WorkDir, "test_kernel")
	KBoot = fp.Join(WorkDir, KBootName)
	KMfg = fp.Join(WorkDir, MfgName)
	KInstaller = fp.Join(WorkDir, "installer.efi")

	InitBin = fp.Join(WorkDir, "init")
	MfgBin = fp.Join(WorkDir, "mfgInit")
	ImgAppsTxz = fp.Join(WorkDir, "img_apps.txz")

	ImgCmds = []string{ImportPath + "/cmd/img/..."}
	UtilCmds = []string{ImportPath + "/cmd/util/..."}
	WinCmds = []string{ImportPath + "/cmd/windows/..."}

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

	BRxz = fp.Join(WorkDir, "combined.cpio.xz")
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
func Pkglist(patterns ...string) ([]string, error) {
	args := []string{"list"}
	args = append(args, patterns...)
	out, err := sh.Output("go", args...)
	if err != nil {
		return nil, err
	}
	return strings.Split(out, "\n"), nil
}
