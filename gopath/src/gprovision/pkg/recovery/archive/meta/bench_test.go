// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package meta

//benchmark native vs C impl

import (
	"bytes"
	"flag"
	"io"
	"os"
	"os/exec"
	"testing"

	"github.com/ulikunitz/xz"
)

var path = flag.String("xzpath", "", "xz >1MB for benchmark")
var verbose = flag.Bool("xzv", false, "verbose")
var megs = flag.Float64("s", 5.0, "megs")

// Native impl docs say it has not been optimized, and unsurprisingly external
// xz is ~7.5x faster. Only use internal version when xz executable is not
// available.
//
// go test -bench . -xzpath /path/to/PRODUCT.Os.Plat.2019-12-10.8682.upd -s 1 -benchtime 10s
// goos: linux
// goarch: amd64
// pkg: gprovision/pkg/recovery/archive/meta
// BenchmarkXZ/internal-16                       20         580680936 ns/op
// BenchmarkXZ/external-16                      200          76486254 ns/op
// PASS
// ok      gprovision/pkg/recovery/archive/meta  35.221s
func BenchmarkXZ(b *testing.B) {
	b.Run("internal", benchmarkIntXZ)
	b.Run("external", benchmarkExtXZ)
}

func benchmarkIntXZ(b *testing.B) {
	f, err := os.Open(*path)
	if err != nil {
		b.Fatal(err)
	}
	defer f.Close()
	buf := &bytes.Buffer{}
	for n := 0; n < b.N; n++ {
		if _, err := f.Seek(0, 0); err != nil {
			b.Errorf("seek: %s", err)
		}
		lr := io.LimitReader(f, int64(*megs*oneM))
		xz, err := xz.NewReader(lr)
		if err != nil {
			b.Fatal(err)
		}
		buf.Truncate(0)
		_, err = buf.ReadFrom(xz)
		if err != nil && err != io.ErrUnexpectedEOF {
			b.Fatal(err)
		}
		if *verbose {
			b.Logf("buf len: %d", buf.Len())
		}
	}
}

func benchmarkExtXZ(b *testing.B) {
	f, err := os.Open(*path)
	if err != nil {
		b.Fatal(err)
	}
	defer f.Close()
	buf := &bytes.Buffer{}
	for n := 0; n < b.N; n++ {
		if _, err := f.Seek(0, 0); err != nil {
			b.Errorf("seek: %s", err)
		}
		xz := exec.Command("xz", "-d")
		xz.Stdin = io.LimitReader(f, int64(*megs*oneM))
		p, err := xz.StdoutPipe()
		if err != nil {
			b.Fatal(err)
		}
		err = xz.Start()
		if err != nil {
			b.Fatal(err)
		}
		buf.Truncate(0)
		_, err = buf.ReadFrom(p)
		if err != nil && err != io.ErrUnexpectedEOF {
			b.Fatal(err)
		}
		if *verbose {
			b.Logf("buf len: %d", buf.Len())
		}
	}
}
