// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package integ

import (
	"gprovision/pkg/log"
	gtst "gprovision/testing"
	"io/ioutil"
	"os"
	fp "path/filepath"
)

// Creates a temp dir, optionally removing old dirs with same prefix. Returns a
// function to defer for clean up. If cleanOld may be true, tests running
// simultaneously must not use same pfx.
func VmDir(tb gtst.TB, pfx string, cleanOld bool) (string, func(tb gtst.TB)) {
	//if pfx is empty and cleanOld true, tried to remove everything in temp dir!
	if len(pfx) == 0 {
		pfx = "gtest-unknown"
		tb.Logf("temp dir prefix unset, now %s", pfx)
	}
	if cleanOld {
		CleanOldDirs(tb, pfx)
	}
	tmpdir, err := ioutil.TempDir(os.Getenv("TEMPDIR"), pfx)
	if err != nil {
		tb.Error(err)
	}
	return tmpdir, func(tb gtst.TB) {
		if !Keep && (!tb.Failed() || OnCI()) {
			tb.Log("cleanup", tmpdir)
			os.RemoveAll(tmpdir)
		} else {
			tb.Log("not deleting", tmpdir)
		}
	}
}

func CleanOldDirs(t gtst.TB, pfx string) {
	t.Log("cleaning up old temp dirs...")
	tmp := os.Getenv("TEMPDIR")
	if len(tmp) == 0 {
		tmp = "/tmp"
	}
	matches, err := fp.Glob(fp.Join(tmp, pfx+"*"))
	if err != nil {
		log.Fatalf("file glob: %s", err)
	}
	for _, m := range matches {
		err = os.RemoveAll(m)
		if err != nil {
			log.Logf("removing %s: %s", m, err)
		}
	}
}
