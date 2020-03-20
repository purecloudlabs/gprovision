// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package integ

import (
	"fmt"
	"gprovision/pkg/common"
	"gprovision/pkg/common/fr"
	"gprovision/pkg/common/strs"
	"io/ioutil"
	"os"
	fp "path/filepath"
	"testing"

	"github.com/u-root/u-root/pkg/qemu"
)

var writePWs9p = func(t *testing.T, rpath string) {
	fname := fp.Join(rpath, "insecure.storage")
	data := []byte(fmt.Sprintf("%s\000%s\000%s", "test1234", "test2345", "test3456"))

	err := ioutil.WriteFile(fname, data, 0600)
	if err != nil {
		t.Fatal(err)
	}
}

//Writes files to a dir that'll be shared via 9p as the recovery volume
func Setup9pRecov(t *testing.T, opts *qemu.Options, tmpdir string, upd []byte, krnl, xlog string) {
	//write files
	rec := fp.Join(tmpdir, strs.RecVolName())
	err := os.MkdirAll(fp.Join(rec, "Image"), 0755)
	if err != nil {
		t.Fatal(err)
	}
	writePWs9p(t, rec)
	if len(upd) > 0 {
		imgName := strs.ImgPrefix() + "2020-03-07.1.upd"
		err = ioutil.WriteFile(fp.Join(rec, "Image", imgName), upd, 0755)
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(krnl) > 0 {
		kdata, err := ioutil.ReadFile(krnl)
		if err != nil {
			t.Fatal(err)
		}
		err = ioutil.WriteFile(fp.Join(rec, strs.BootKernel()), kdata, 0644)
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(xlog) > 0 {
		fr.SetXLog(xlog)
		p := common.PatherMock(rec)
		fr.SetUnit(common.Unit{Rec: &p})
		err = fr.Persist()
		if err != nil {
			t.Fatal(err)
		}
	}

	//add 9p device
	dir := qemu.P9Directory{
		Dir: rec,
		Tag: strs.RecVolName(),
	}
	opts.Devices = append(opts.Devices, dir)
}
