// Copyright (C) 2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

// +build gofuzz

package uefi

/*
go get github.com/dvyukov/go-fuzz/go-fuzz
go get github.com/dvyukov/go-fuzz/go-fuzz-build

go-fuzz-build -func FuzzParseFilePathList github.com/purecloudlabs/gprovision/pkg/hw/uefi
go-fuzz -bin=./uefi-fuzz.zip -workdir=fuzz
...
*/

func FuzzParseFilePathList(data []byte) int {
	_, err := ParseFilePathList(data)
	if err != nil {
		return 0
	}
	return 1
}
