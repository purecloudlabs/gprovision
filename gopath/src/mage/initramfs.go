// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

// +build mage

package main

import (
	"context"
	"fmt"
	"gprovision/testing/util"
	"io/ioutil"
	"os"
	"os/exec"
	fp "path/filepath"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
	"github.com/magefile/mage/target"
)

type Initramfs mg.Namespace

//initramfs used during mfg process, containing more files than normal boot kernel
func (Initramfs) Mfg(ctx context.Context) error {
	mg.CtxDeps(ctx, Initramfs.Combined_cpio, Bins.MfgInit, workdir)
	update, err := target.Dir(InitramfsMfg, BRcpio, MfgBin, fp.Join(RepoRoot, "initramfs_mfg"), fp.Join(RepoRoot, "initramfs"))
	if err != nil {
		return err
	}
	if !update {
		fmt.Println("skipping build of mfg cpio")
		return nil
	}
	tmpdir, err := ioutil.TempDir(WorkDir, "mfg-initramfs")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpdir)
	files, err := util.FileList("initramfs")
	if err != nil {
		return err
	}
	files2, err := util.FileList("initramfs_mfg")
	if err != nil {
		return err
	}
	files = append(files, files2...)
	files = append(files, fmt.Sprintf("%s:init", MfgBin))
	combined, err := os.Open(BRcpio)
	if err != nil {
		return err
	}
	defer combined.Close()
	return util.CreateInitramfs(InitramfsMfg, tmpdir, combined, files, false)
}

//initramfs used on customer machines, containing factory restore + normal boot logic
func (Initramfs) Boot(ctx context.Context) error {
	mg.CtxDeps(ctx, Initramfs.Combined_cpio, Bins.NormalInit, workdir)
	update, err := target.Dir(InitramfsBoot, BRcpio, InitBin, fp.Join(RepoRoot, "initramfs"))
	if err != nil {
		return err
	}
	if !update {
		fmt.Println("skipping build of normal boot cpio")
		return nil
	}
	tmpdir, err := ioutil.TempDir(WorkDir, "fr-initramfs")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpdir)
	files, err := util.FileList("initramfs")
	if err != nil {
		return err
	}
	files = append(files, fmt.Sprintf("%s:init", InitBin))

	// Need CA certs to use sumo collector. Certs from the ci instance will be
	// quite fresh, given the frequency with which those instances are replaced.
	// https://serverfault.com/questions/620003/difference-between-ca-bundle-crt-and-ca-bundle-trust-crt#
	caBundle := getCAbundle()
	if caBundle == "" {
		return fmt.Errorf("cannot find CA certs to use")
	}
	ca, err := fp.EvalSymlinks(caBundle)
	if err != nil {
		return err
	}
	files = append(files, fmt.Sprintf("%s:%s", ca, caBundle))

	combined, err := os.Open(BRcpio)
	if err != nil {
		return err
	}
	defer combined.Close()
	return util.CreateInitramfs(InitramfsBoot, tmpdir, combined, files, false)
}

//extract combined.cpio from combined.cpio.xz
func (Initramfs) Combined_cpio(ctx context.Context) error {
	mg.CtxDeps(ctx, Initramfs.Combined_xz, workdir)
	changed, err := target.Path(BRcpio, BRxz)
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}
	f, err := os.Create(BRcpio)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			os.Remove(BRcpio)
		}
	}()
	defer f.Close()
	_, err = sh.Exec(nil, f, os.Stderr, "xz", "-dc", BRxz)
	return err
}

//download xz from blobstore
func (Initramfs) Combined_xz() error {
	mg.Deps(paths)
	_, err := os.Stat(BRxz)
	if err == nil {
		return nil
	}
	err = blobcp(BlobstoreBRCombined, BRxz, false)
	if err != nil {
		fmt.Println("error downloading combined.cpio.xz from blobstore - if it does not exist, rebuild with initramfs:rebuild_xz and manually upload")
	}
	return err
}

//rebuild combined.cpio.xz using buildroot (slow!)
func (Initramfs) Rebuild_xz(ctx context.Context) error {
	//see brx/README and brx/Makefile
	mg.CtxDeps(ctx, paths)
	br := fp.Join(WorkDir, "buildroot")
	_, err := os.Stat(br)
	if err == nil {
		fmt.Printf("buildroot dir %s exists, assuming it contains the correct version\n", br)
	} else {
		fmt.Println("clone buildroot...")
		err := sh.Run("git", "clone", "--branch", "2017.02", "https://github.com/buildroot/buildroot", br)
		if err != nil {
			return err
		}
	}
	err = sh.Copy(fp.Join(br, ".config"), fp.Join(RepoRoot, "brx/buildroot.config"))
	if err != nil {
		return err
	}
	env := make(map[string]string)
	env["BUILDROOT"] = br
	fmt.Println("run buildroot. this will take a *very* long time...")
	//do not use `-j` with make - buildroot handles parallelism itself
	err = sh.RunWith(env, "make", "-C", fp.Join(RepoRoot, "brx"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "buildroot failed. check for necessary dependencies - see docs in", br)
		return err
	}
	fmt.Println("compress...")
	in, err := os.Open(fp.Join(RepoRoot, "brx", "combined.cpio"))
	if err != nil {
		return err
	}
	defer in.Close()
	if err != nil {
		return err
	}
	out, err := os.Create(BRxz)
	defer out.Close()
	xz := exec.CommandContext(ctx, "xz")
	xz.Stdin = in
	xz.Stdout = out
	err = xz.Run()
	return err
}

func getCAbundle() string {
	// Copied from crypto/x509/root_linux.go. Unfortunately there doesn't seem
	// to be a way to get the relevant info from that package as-is.
	var certFiles = []string{
		"/etc/ssl/certs/ca-certificates.crt",                // Debian/Ubuntu/Gentoo etc.
		"/etc/pki/tls/certs/ca-bundle.crt",                  // Fedora/RHEL 6
		"/etc/ssl/ca-bundle.pem",                            // OpenSUSE
		"/etc/pki/tls/cacert.pem",                           // OpenELEC
		"/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem", // CentOS/RHEL 7
	}
	for _, f := range certFiles {
		_, err := os.Stat(f)
		if err == nil {
			return f
		}
	}
	return ""
}
