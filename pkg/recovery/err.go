// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package recovery

import (
	"github.com/purecloudlabs/gprovision/pkg/common/rkeep"
	"github.com/purecloudlabs/gprovision/pkg/hw/power"
	"github.com/purecloudlabs/gprovision/pkg/log"
)

var RecFatal = log.FailAction{
	MsgPfx: "ERROR: ",
	Pre: func(f string, va ...interface{}) {
		if log.GetPrefix() == "test" {
			panic("Fatalf called from 'go test'")
		}
		rkeep.ReportFailure(log.GetPrefix() + " failed, rebooting...")
	},
	Terminator: func() {
		power.Reboot(false)
	},
}
