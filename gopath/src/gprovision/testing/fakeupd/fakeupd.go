// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

// Package fakeupd creates a fake .upd file with enough content to keep factory
// restore happy and to boot to a point where it can print a message.
//
// Used in integration testing.
package fakeupd

import (
	"archive/tar"
	"bytes"
	"fmt"
	"gprovision/pkg/common/strs"
	"gprovision/pkg/init/consts"
	"io/ioutil"
	"os"
	"os/exec"
	fp "path/filepath"
	"strings"
)

const (
	InitRunning   = "fake init running..."
	ChpassRunning = "fake chpass running..."
	Bye           = "bye"
)

//create a tar.xz with sha256
func Make(tmpdir, kern, logger string) ([]byte, error) {
	ibuf := &bytes.Buffer{}
	u := &upd{
		tr:     tar.NewWriter(ibuf),
		logger: logger,
	}
	//create /boot; dirs have header only and no body
	err := u.createDirs("boot", "etc", "etc/systemd", "dev", "usr", "usr/lib")
	if err != nil {
		return nil, wraperr("dirs", err)
	}

	//write kernel
	err = u.addKernel(kern)
	if err != nil {
		return nil, wraperr("kernel", err)
	}
	//write fake systemd init
	err = u.addFakeSysd()
	if err != nil {
		return nil, wraperr("fakeSysd", err)
	}
	//write fake chpasswd
	err = u.addFakeChpasswd()
	if err != nil {
		return nil, wraperr("fakeChpasswd", err)
	}
	//finish up
	err = u.tr.Close()
	if err != nil {
		return nil, wraperr("close tar", err)
	}
	xz := exec.Command("xz", "-C", "sha256", "-0")
	xz.Stdin = ibuf
	obuf := &bytes.Buffer{}
	xz.Stdout = obuf
	err = xz.Run()
	if err != nil {
		return nil, wraperr("xz", err)
	}
	return obuf.Bytes(), nil
}

type upd struct {
	tr        *tar.Writer
	fakeMulti string //used by addFakeBinary - further adds translate to symlinks
	logger    string
}

func (u *upd) addKernel(bootk string) error {
	kdata, err := ioutil.ReadFile(bootk)
	if err != nil {
		return wraperr("read", err)
	}
	err = u.tr.WriteHeader(&tar.Header{
		Name: "boot/" + strs.BootKernel(),
		Size: int64(len(kdata)),
		Mode: 0644,
	})
	if err != nil {
		return wraperr("write hdr", err)
	}
	_, err = u.tr.Write(kdata)
	if err != nil {
		return wraperr("write data", err)
	}
	return nil
}

func (u *upd) createDirs(dirs ...string) error {
	for _, d := range dirs {
		if !strings.HasSuffix(d, "/") {
			d += "/"
		}
		err := u.tr.WriteHeader(&tar.Header{
			Name: d,
			Mode: 0777,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

//in fake .upd, writes a systemd "init" where our init expects to find it
func (u *upd) addFakeSysd() error {
	return u.addFakeBinary(consts.RealInit)
}

func (u *upd) addFakeChpasswd() error {
	return u.addFakeBinary("/bin/chpasswd")
}

//first call adds binary, others add symlinks
func (u *upd) addFakeBinary(dest string) error {
	if len(u.fakeMulti) > 0 {
		err := u.tr.WriteHeader(&tar.Header{
			Name:     strings.TrimPrefix(dest, "/"),
			Linkname: u.fakeMulti,
			Typeflag: tar.TypeSymlink,
		})
		return err
	}
	u.fakeMulti = dest
	tmpdir, err := ioutil.TempDir("", "test-gprov-fake"+fp.Base(dest))
	if err != nil {
		return wraperr("tmpdir", err)
	}
	defer os.RemoveAll(tmpdir)
	//build
	out := fp.Join(tmpdir, fp.Base(dest))
	var linkarg string
	if u.logger != "" {
		linkarg = fmt.Sprintf("-X 'main.logger=%s'", u.logger)
	}
	src := fp.Join(strings.Split(os.Getenv("GOPATH"), ":")[0], "src/gprovision/cmd/dummy")
	gb := exec.Command("go", "build", "-o", out, "-ldflags", linkarg, src)
	gb.Env = append(os.Environ(), "CGO_ENABLED=0")
	output, err := gb.CombinedOutput()
	if err != nil {
		fmt.Printf("running %#v:\n%s\n", gb.Args, output)
		return wraperr("gobuild", err)
	}
	//read
	bin, err := ioutil.ReadFile(out)
	if err != nil {
		return wraperr("read binary", err)
	}
	//create dir in tar
	dir := strings.TrimPrefix(fp.Dir(dest), "/") + "/"
	err = u.tr.WriteHeader(&tar.Header{
		Name: dir,
		Mode: 0777,
	})
	if err != nil {
		return wraperr("dir header", err)
	}
	//add binary to tar
	err = u.tr.WriteHeader(&tar.Header{
		Name: strings.TrimPrefix(dest, "/"),
		Size: int64(len(bin)),
		Mode: 0755,
	})
	if err != nil {
		return wraperr("binary hdr", err)
	}
	_, err = u.tr.Write(bin)
	if err != nil {
		return wraperr("write binary", err)
	}
	return nil
}

type wrappedErr struct {
	desc string
	orig error
}

func (w *wrappedErr) Error() string {
	return fmt.Sprintf("%s: %s", w.desc, w.orig)
}

func wraperr(desc string, err error) *wrappedErr {
	return &wrappedErr{
		desc: desc,
		orig: err,
	}
}
