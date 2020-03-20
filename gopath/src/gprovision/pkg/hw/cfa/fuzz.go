// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

// +build gofuzz

package cfa

import (
	"bytes"
)

/*
go get github.com/dvukov/go-fuzz/go-fuzz
go get github.com/dvukov/go-fuzz/go-fuzz-build

go-fuzz-build -func FuzzGetPacket gprovision/pkg/hw/cfa
go-fuzz -bin=./cfa-fuzz.zip -workdir=fuzz
...
*/

func FuzzGetPacket(data []byte) int {
	buf := bytes.NewBuffer(data)
	nf := &nopFlusher{r: buf}
	p, err := GetPacket(nf, false, false)

	if err != nil && p == nil {
		return 0
	}
	return 1
}
