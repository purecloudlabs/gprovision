// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package qa

import (
	"testing"
)

func TestReadCpu(t *testing.T) {
	var c CPUInfo
	c.Read()
	t.Logf("%v\n", c)
}
