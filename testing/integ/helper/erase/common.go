// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package main

import (
	"encoding/binary"
	"math/rand"
)

const (
	oneM = 1024 * 1024
	oneG = oneM * 1024
)

//creates a 1MB reproducible but not-quite-trivial pattern.
func pat1m(idx int64) []byte {
	meg := make([]byte, oneM)
	cursor := meg
	var p256 [256]byte
	for i := range p256 {
		p256[i] = 255 - byte(i)
	}
	for len(cursor) > 0 {
		copy(cursor[:], p256[:])
		if len(cursor) < len(p256) {
			break
		}
		cursor = cursor[len(p256):]
		var quad [4]byte
		binary.LittleEndian.PutUint32(quad[:], uint32(idx))
		copy(cursor[:], quad[:])
		idx++
		if len(cursor) < 4 {
			break
		}
		cursor = cursor[4:]
	}
	return meg
}

//rng seeded with disk size - repeatable pattern, but not trivially predictable
func offsetSeq(siz int64) []int64 {
	rnd := rand.NewSource(siz)
	var lastoff int64
	var offsets []int64
	for {
		// clamp offset variation
		//                        mb   0x100000
		//                        gb 0x40000000
		off := rnd.Int63() & 0x00000000ffe00000
		if off == 0 {
			off = oneG
		}
		lastoff += off
		if lastoff >= siz {
			break
		}
		offsets = append(offsets, lastoff)
	}
	return offsets
}
