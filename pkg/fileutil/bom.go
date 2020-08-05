// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package fileutil

import (
	"bytes"
	"fmt"
	"io"
)

/* http://www.unicode.org/faq/utf_bom.html#BOM
Bytes		Encoding Form
00 00 FE FF  UTF-32, big-endian
FF FE 00 00  UTF-32, little-endian
FE FF        UTF-16, big-endian
FF FE        UTF-16, little-endian
EF BB BF     UTF-8
*/

type UtfVariant uint

const (
	None UtfVariant = iota
	Utf32be
	Utf32le
	Utf16be
	Utf16le
	Utf8
)

//Check for UTF byte order mark; if present, seeks past it
func DetectBOM(fd io.ReadSeeker) (v UtfVariant, err error) {
	pos, err := fd.Seek(0, 1)
	if err != nil {
		return
	}
	if pos != 0 {
		err = fmt.Errorf("must start at beginning of stream, but position is %d", pos)
		return
	}
	var bom [4]byte
	_, err = io.ReadFull(fd, bom[:])
	if err != nil {
		return
	}
	if bytes.Equal(bom[:], []byte{0x00, 0x00, 0xfe, 0xff}) {
		v = Utf32be
		return
	}
	if bytes.Equal(bom[:], []byte{0xff, 0xfe, 0x00, 0x00}) {
		v = Utf32le
		return
	}
	if bytes.Equal(bom[:2], []byte{0xfe, 0xff}) {
		_, err = fd.Seek(2, 0)
		if err == nil {
			v = Utf16be
		}
		return
	}
	if bytes.Equal(bom[:2], []byte{0xff, 0xfe}) {
		_, err = fd.Seek(2, 0)
		if err == nil {
			v = Utf16le
		}
		return
	}
	if bytes.Equal(bom[:3], []byte{0xef, 0xbb, 0xbf}) {
		_, err = fd.Seek(3, 0)
		if err == nil {
			v = Utf8
		}
		return
	}
	_, err = fd.Seek(0, 0)
	return
}
