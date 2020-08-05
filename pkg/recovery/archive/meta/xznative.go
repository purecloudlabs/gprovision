// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

// +build !linux !release

// Above means (NOT linux) OR (NOT release); for linux, assume xz is available.
// Still build on linux for non-release builds (i.e. testing, benchmarks).

package meta

import (
	"io"
	"io/ioutil"
	"os/exec"

	"github.com/ulikunitz/xz"
)

// Decompress xz with external executable if present, fall back to native impl.
func unxzr(rdr io.Reader) (io.ReadCloser, func(), error) {
	if haveXz() {
		return externalUnxz(rdr)
	} else {
		rc, err := nativeUnxz(rdr)
		return rc, func() {}, err
	}
}

// This version uses native xz impl for decompression. Native impl is not
// optimized and is ~7.5x slower than using xz executable.
func nativeUnxz(f io.Reader) (io.ReadCloser, error) {
	reader, err := xz.NewReader(f)
	//returned value is a Reader but we want a ReadCloser for consistency.
	return ioutil.NopCloser(reader), err
}

//true if xz or xz.exe exists
func haveXz() bool {
	_, err := exec.LookPath("xz") // windows impl of LookPath will append .exe
	return err == nil
}
