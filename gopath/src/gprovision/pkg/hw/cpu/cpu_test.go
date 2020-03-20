// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package cpu

import (
	"runtime"
	"testing"
)

func TestSysCpuMask(t *testing.T) {
	setCPUInfo()
	n := Count()
	m := Mask()
	r := uint16(runtime.NumCPU())

	t.Logf("m=%x, n=%d, r=%d\n", m, n, r)
	if m == 0 {
		t.Errorf("must be > 0")
	}
	if n != r {
		t.Errorf("runtime reports different number of cpus (%d) than we do (%d)", r, n)
	}
}
