// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package main

import (
	"bytes"
	"testing"
)

//func pat1m(idx int64) []byte
func TestPat1m(t *testing.T) {
	p := pat1m(1024 * 1024 * 1024 * 4)
	p2 := pat1m(1024 * 1024 * 35)
	if bytes.Equal(p, p2) {
		t.Error("equal")
		t.Logf("\np1=...% 02x...\np2=...% 02x...", p[250:260], p2[250:260])
	}
}

//func offsetSeq(siz int64) []int64
func TestOffsetSeq(t *testing.T) {
	//ensure different volume sizes produce different offsets
	var outs [][]int64
	for _, td := range []struct {
		name string
		in   int64
		ln   int
	}{
		{name: "0", in: oneG * 88, ln: 45},
		{name: "1", in: oneG*88 - 512, ln: 44},
		{name: "2", in: oneG*88 + 512, ln: 48},
		{name: "3", in: oneG * 256, ln: 121},
		{name: "4", in: oneG * 880, ln: 431},
	} {
		t.Run(td.name, func(t *testing.T) {
			out := offsetSeq(td.in)
			if len(out) != td.ln {
				t.Errorf("bad len - want %d got %d", td.ln, len(out))
				t.Logf("% 02x", out)
			}
			for n, o := range out {
				if n > 0 && o <= out[n-1] {
					t.Errorf("seq must increase: %d, % 02x", n, out)
				}
				if o >= td.in {
					t.Errorf("exceeds device size at %d: %d (%d) % 02x", n, o, td.in, out)
				}
				if o%oneM != 0 {
					t.Errorf("bad offset %d (remainder %d) at %d % 02x", o, o%oneM, n, out)
				}
			}
			outs = append(outs, out)
		})
	}
	//check if any sequences are the same when truncated to same len
	for i, io := range outs {
		for j, jo := range outs[i+1:] {
			eq := true
			for n := range io {
				if len(jo) > n && jo[n] != io[n] {
					eq = false
					break
				}
			}
			if eq {
				t.Errorf("equality between %d and %d:\n% 02x\n% 02x", i, j+i+1, io, jo)
			}
		}
	}
}
