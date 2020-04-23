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
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	fp "path/filepath"
	"strings"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

//downloads/extracts qemu tarball
func qemu(ctx context.Context) error {
	mg.CtxDeps(ctx, paths)
	os.Mkdir(QDir, 0755)
	_, err := os.Stat(QemuSys)
	if err == nil {
		//qemu-system is present, assume others are as well
		return nil
	}
	_, present := os.LookupEnv("UROOT_QEMU")
	if present {
		//env var already set, assume qemu-system and qemu-img are in PATH
		QemuImg = "qemu-img"
		return nil
	}
	tball := fp.Join(WorkDir, "qemu.txz")
	if _, err := os.Stat(tball); err == nil {
		//tarball exists
		return wrapIf("extracting tarball", sh.Run("tar", "xJf", tball, "-C", QDir))
	}
	//try to get from blobstore
	if err = blobcp(BlobstoreQemu, tball, false); err == nil {
		return wrapIf("extracting tarball", sh.Run("tar", "xJf", tball, "-C", QDir))
	}
	return fmt.Errorf("could not retrieve qemu.txz from blobstore")
}

func blobrm(path string) error {
	if len(BlobstorePrefix) == 0 {
		fmt.Println("skipping blob op, no prefix set")
		return nil
	}
	u, err := url.Parse(BlobstorePrefix)
	if err != nil {
		fmt.Printf("parsing %s: %s\n", BlobstorePrefix, err)
		return err
	}
	switch u.Scheme {
	case "s3":
		err = sh.Run("aws", "s3", "rm", "--recursive", path)
		if err != nil {
			fmt.Printf("recursively cleaning %s: %s", path, err)
			return err
		}
	default:
		fmt.Printf("unsupported blobstore %s\n", u.Scheme)
		return os.ErrInvalid
	}
	return nil
}

func blobcp(src, dest string, acl bool) error {
	if len(BlobstorePrefix) == 0 {
		fmt.Println("skipping blob op, no prefix set")
		return nil
	}
	u, err := url.Parse(BlobstorePrefix)
	if err != nil {
		fmt.Printf("parsing %s: %s\n", BlobstorePrefix, err)
		return err
	}
	switch u.Scheme {
	case "s3":
		cp := exec.Command("aws", "s3", "cp", src, dest)
		if acl {
			cp.Args = append(cp.Args, "--acl", "bucket-owner-full-control")
		}
		out, err := cp.CombinedOutput()
		if err == nil {
			return nil
		}
		fmt.Printf("%v failed with %s\noutput:\n%s\n", cp.Args, err, out)
		return err
	case "https":
		if strings.HasPrefix(dest, u.Scheme) {
			fmt.Println("cannot upload to https, ignoring - ", dest)
			return nil
		}
		var resp *http.Response
		if resp, err = http.Get(src); err != nil {
			return err
		}
		var out *os.File
		if out, err = os.Create(dest); err != nil {
			return err
		}
		if _, err = io.Copy(out, resp.Body); err != nil {
			return err
		}
	default:
		fmt.Printf("unsupported blobstore %s\n", u.Scheme)
		return os.ErrInvalid
	}
	return nil
}

type wrappedErr struct {
	sub error
	msg string
}

func (w *wrappedErr) Error() string {
	if w.sub == nil {
		return w.msg
	}
	if !strings.Contains(w.msg, "%s") {
		return fmt.Sprintf("%s: %s", w.msg, w.sub)
	}
	return fmt.Sprintf(w.msg, w.sub)
}
func wrapIf(msg string, err error) error {
	if err == nil {
		return nil
	}
	return &wrappedErr{err, msg}
}

type wrappedExecErr struct {
	sub  error
	args []string
	out  string
}

func (w *wrappedExecErr) Error() string {
	str := fmt.Sprintf("running %#v: %s", w.args, w.sub)
	if len(w.out) > 0 {
		str += fmt.Sprintf("\noutput:\n%s", w.out)
	}
	return str
}

//set up env vars for a run of qemu
func qemuEnv(kernel string, uefi bool) []string {
	var env []string
	env = append(env, fmt.Sprintf("%s=%s", "INFRA_ROOT", RepoRoot))
	qenv := os.Getenv("UROOT_QEMU")
	if qenv == "" {
		args := []string{
			QemuSys,
			"-smp", "1",
			"-cpu", "qemu64",
			"-L", BiosDir,
		}
		qenv = strings.Join(args, " ")
	}
	env = append(env, fmt.Sprintf("%s=%s", "UROOT_QEMU", qenv))
	env = append(env, fmt.Sprintf("%s=%s", "QEMU_IMG", QemuImg))
	if uefi {
		env = append(env, fmt.Sprintf("%s=%s", "OVMF_DIR", OvmfDir))
	}
	env = append(env, fmt.Sprintf("%s=%s", "UROOT_KERNEL", kernel))
	var printEnv string
	for _, e := range env {
		printEnv += fmt.Sprintf("%q ", e)
	}
	fmt.Printf("env:\n%s\n", printEnv)
	return env
}
