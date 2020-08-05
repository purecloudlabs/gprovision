// Copyright (C) 2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

// +build !release

package history

//Writes arbitrary records to history file, for use in integ tests.
func WriteArbitraryHistory(rec string, res ResultList) {
	SetRoot(rec)
	write(res)
}
