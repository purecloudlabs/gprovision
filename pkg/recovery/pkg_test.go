// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package recovery

import (
	"strings"
	"testing"

	"github.com/purecloudlabs/gprovision/pkg/common/strs"
)

//func Hostify(id string) string
func TestHostify(t *testing.T) {
	io := [][]string{
		{"sadfl.", strs.HostPrefix() + "sadfl0"},
		{"AFDS", strs.HostPrefix() + "afds"},
		{"#$%@%%", strs.HostPrefix() + "-----0"},
		{"", strings.TrimSuffix(strs.HostPrefix(), "-") + "0"},
	}
	for _, p := range io {
		res := Hostify(p[0])
		if res != p[1] {
			t.Errorf("got %s, wanted %s", res, p[1])
		}
	}
}
