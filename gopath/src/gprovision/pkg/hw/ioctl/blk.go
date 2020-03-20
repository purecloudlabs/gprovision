// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package ioctl

//BLKSSZGET
func BlkGetSectorSize(f FDer) (uint64, error) {
	BLKSSZGET := 0x1268
	s, err := Ioctl1(f.Fd(), BLKSSZGET)
	return uint64(s), err
}

//BLKGETSIZE64
func BlkGetSize64(f FDer) (uint64, error) {
	BLKGETSIZE64 := 0x80081272
	return Ioctl1(f.Fd(), BLKGETSIZE64)
}
