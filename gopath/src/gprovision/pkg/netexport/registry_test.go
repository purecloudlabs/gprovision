// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package netexport

import (
	"testing"
)

//func guidStrFromRegBin(binGuid []byte) string
func TestGuidFromBin(t *testing.T) {
	guid1 := []byte{0xa2, 0xd1, 0x1b, 0x1d, 0xd9, 0x0f, 0xe9, 0x41, 0xbb, 0xb5, 0xa9, 0x8b, 0xac, 0x57, 0x0b, 0x2a}
	guid2 := []byte{0x3e, 0x14, 0xbe, 0xcf, 0x9e, 0x5e, 0x25, 0x46, 0xa5, 0x00, 0xc3, 0xf0, 0x36, 0x20, 0x04, 0x11}
	want1 := "{1D1BD1A2-0FD9-41E9-BBB5-A98BAC570B2A}"
	want2 := "{CFBE143E-5E9E-4625-A500-C3F036200411}"
	out1 := guidStrFromRegBin(guid1)
	if out1 != want1 {
		t.Errorf("bad guid decode - want\n%s, got \n%s\n", want1, out1)
	}
	out2 := guidStrFromRegBin(guid2)
	if out2 != want2 {
		t.Errorf("bad guid decode - want\n%s, got \n%s\n", want2, out2)
	}
}

//func maskFromString(m string) net.IPMask
func TestMaskFromString(t *testing.T) {
	s1 := "255.0.0.0"
	s2 := "255.255.0.0"
	m1 := maskFromString(s1)
	m2 := maskFromString(s2)
	bits1, _ := m1.Size()
	if bits1 != 8 {
		t.Errorf("mismatch - got %s, want %s", m1.String(), s1)
	}
	bits2, _ := m2.Size()
	if bits2 != 16 {
		t.Errorf("mismatch - got %s, want %s", m2.String(), s2)
	}
}
