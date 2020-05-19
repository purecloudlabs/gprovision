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
