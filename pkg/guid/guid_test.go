// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package guid

import (
	"bytes"
	"testing"
)

//func MSStrFromRegBin(binGuid []byte) string
func TestGuidFromBin(t *testing.T) {
	guid1 := []byte{0xa2, 0xd1, 0x1b, 0x1d, 0xd9, 0x0f, 0xe9, 0x41, 0xbb, 0xb5, 0xa9, 0x8b, 0xac, 0x57, 0x0b, 0x2a}
	guid2 := []byte{0x3e, 0x14, 0xbe, 0xcf, 0x9e, 0x5e, 0x25, 0x46, 0xa5, 0x00, 0xc3, 0xf0, 0x36, 0x20, 0x04, 0x11}
	want1 := "{1D1BD1A2-0FD9-41E9-BBB5-A98BAC570B2A}"
	want2 := "{CFBE143E-5E9E-4625-A500-C3F036200411}"
	out1 := MSStrFromRegBin(guid1)
	if out1 != want1 {
		t.Errorf("bad guid decode - want\n%s, got \n%s\n", want1, out1)
	}
	out2 := MSStrFromRegBin(guid2)
	if out2 != want2 {
		t.Errorf("bad guid decode - want\n%s, got \n%s\n", want2, out2)
	}
}

func TestEncodeDecode(t *testing.T) {
	in := []byte{0xCD, 0x5C, 0x63, 0x81, 0x4F, 0x1B, 0x3F, 0x4D, 0xB7, 0xB7, 0xF7, 0x8A, 0x5B, 0x02, 0x9F, 0x35}
	want := "81635ccd-1b4f-4d3f-b7b7-f78a5b029f35"

	var m MixedGuid
	copy(m[:], in)
	std := m.ToStdEnc()
	got := std.String()

	if got != want {
		t.Errorf("mismatch\n%s\n%s", want, got)
	}
	guid := FromStdEnc(std)
	if !bytes.Equal(guid[:], in) {
		t.Errorf("mismatch\n%x\n%x", in, guid)
	}
	t.Logf("%s %s", std.Variant(), std.Version())
}
