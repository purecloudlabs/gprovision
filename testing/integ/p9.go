// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package integ

import (
	"fmt"
	"io/ioutil"
	"os"
	fp "path/filepath"
	"strings"
	"testing"

	"github.com/u-root/u-root/pkg/qemu"

	"github.com/purecloudlabs/gprovision/pkg/common"
	"github.com/purecloudlabs/gprovision/pkg/common/fr"
	"github.com/purecloudlabs/gprovision/pkg/common/strs"
	"github.com/purecloudlabs/gprovision/pkg/recovery/history"
)

var writePWs9p = func(t *testing.T, rpath string) {
	fname := fp.Join(rpath, "insecure.storage")
	data := []byte(fmt.Sprintf("%s\000%s\000%s", "test1234", "test2345", "test3456"))

	err := ioutil.WriteFile(fname, data, 0600)
	if err != nil {
		t.Fatal(err)
	}
}

type P9RecovOpts struct {
	T            *testing.T
	Qopts        *qemu.Options
	Tmpdir       string
	Upd          []byte
	Krnl, Xlog   string
	BadHistory   bool
	rec, imgName string
}

//Writes files to a dir that'll be shared via 9p as the recovery volume
func (p9 *P9RecovOpts) Setup() {
	//write files
	p9.rec = fp.Join(p9.Tmpdir, strs.RecVolName())
	err := os.MkdirAll(fp.Join(p9.rec, "Image"), 0755)
	if err != nil {
		p9.T.Fatal(err)
	}
	writePWs9p(p9.T, p9.rec)
	if len(p9.Upd) > 0 {
		p9.imgName = strs.ImgPrefix() + "2020-03-07.1.upd"
		err = ioutil.WriteFile(fp.Join(p9.rec, "Image", p9.imgName), p9.Upd, 0755)
		if err != nil {
			p9.T.Fatal(err)
		}
	}
	if len(p9.Krnl) > 0 {
		kdata, err := ioutil.ReadFile(p9.Krnl)
		if err != nil {
			p9.T.Fatal(err)
		}
		err = ioutil.WriteFile(fp.Join(p9.rec, strs.BootKernel()), kdata, 0644)
		if err != nil {
			p9.T.Fatal(err)
		}
	}
	if len(p9.Xlog) > 0 {
		fr.SetXLog(p9.Xlog)
		p := common.PatherMock(p9.rec)
		fr.SetUnit(common.Unit{Rec: &p})
		err = fr.Persist()
		if err != nil {
			p9.T.Fatal(err)
		}
	}

	//add 9p device
	dir := qemu.P9Directory{
		Dir: p9.rec,
		Tag: strs.RecVolName(),
	}
	p9.Qopts.Devices = append(p9.Qopts.Devices, dir)

	if p9.BadHistory {
		// {"ImageResults":[{"Image":"testfile","ImagingAttempts":1,"Notes":["Imaging @ 2020-06-16T22:15:14Z, success: true"]}]}
		imname := strings.TrimSuffix(p9.imgName, ".upd")
		imname = strings.Replace(imname, ".HW.", ".QEMU.", 1) //in history file, uses actual platform not generic
		res := history.ImageResult{
			Image:           imname,
			ImagingAttempts: history.MaxFailuresPerImg,
			ImagingFailures: history.MaxFailuresPerImg,
			BootAttempts:    history.MaxFailuresPerImg,
			BootFailures:    history.MaxFailuresPerImg,
		}
		history.WriteArbitraryHistory(p9.rec, history.ResultList{&res})
	}
}
