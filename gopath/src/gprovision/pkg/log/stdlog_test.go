// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package log_test

import (
	ilog "gprovision/pkg/log"
	"log"
	"testing"
)

func TestStdlog(t *testing.T) {
	log.Print("test output")
	ilog.AddConsoleLog(0)
	ilog.FlushMemLog()
	ilog.AdaptStdlog(nil, 0, true)
	//log uses a mutex to guard state. this will hang if we fall afoul of that.
	log.Print("test output 2")
}
