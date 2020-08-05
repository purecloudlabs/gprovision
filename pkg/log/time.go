// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package log

import (
	"time"
)

//Format: yyyymmdd_hhmm
const DefaultTimestampLayout = "20060102_1504"

var TimestampLayout = DefaultTimestampLayout

//Returns a string containing a timestamp like TimestampLayout.
func Timestamp() string {
	t := time.Now()
	return t.Format(TimestampLayout)
}
