// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

// +build release

package serial

//release builds: tracef is a no-op.
func tracef(_ string, _ ...interface{}) func(string, ...interface{}) {
	return func(string, ...interface{}) {}
}

//no-op in release builds
func Debug(_, _ bool) {}
