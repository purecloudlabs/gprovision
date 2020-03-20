// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package meta

import (
	"gprovision/pkg/log"
	"io"
	"os"
	"os/exec"
)

//Reads from path, passing through an xz decompressor. Returned function is for
// cleanup. Underlying impl depends on build tags and OS - see xznative.go
// and xzexternal.go.
func unxz(path string) (io.ReadCloser, func(), error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, func() {}, err
	}
	rc, cleanup, err := unxzr(f)
	if err != nil {
		f.Close()
		return nil, func() {}, err
	}
	return rc, func() { cleanup(); f.Close() }, nil
}

// Decompress with xz executable. Faster than native. Returned function is for
// cleanup.
func externalUnxz(f io.Reader) (io.ReadCloser, func(), error) {
	xz := exec.Command("xz", "-d")
	xz.Stdin = f
	p, err := xz.StdoutPipe()
	if err != nil {
		return nil, func() {}, err
	}
	err = xz.Start()
	if err != nil {
		return nil, func() {}, err
	}
	return p, func() {
		err = xz.Process.Kill()
		if err != nil {
			log.Logf("kill xz: %s", err)
		}
	}, nil
}
