// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package mfg

import (
	"fmt"
	"time"

	"github.com/purecloudlabs/gprovision/pkg/common/rkeep"
	"github.com/purecloudlabs/gprovision/pkg/hw/beep"
	"github.com/purecloudlabs/gprovision/pkg/hw/cfa"
	"github.com/purecloudlabs/gprovision/pkg/hw/ipmi/uid"
	"github.com/purecloudlabs/gprovision/pkg/hw/power"
	"github.com/purecloudlabs/gprovision/pkg/log"
)

var MfgFatal = log.FailAction{
	MsgPfx: "ERROR: ",
	Pre: func(f string, va ...interface{}) {
		s := fmt.Sprintf(f, va...)
		if log.GetPrefix() == "test" {
			panic("Fatalf called from 'go test'")
		}
		rkeep.ReportFailure(log.GetPrefix() + " failed...")
		done := make(chan struct{})
		go func() { _ = beep.BeepUntil(done, time.Second*2) }()
		go func() { _ = uid.BlinkUntil(done, 4) }()
		_ = cfa.DefaultLcd.BlinkMsg(s, cfa.Fade, time.Second*2, 48*time.Hour)
		close(done)
		rkeep.ReportFailure(log.GetPrefix() + " failed, rebooting...")
	},
	Terminator: func() {
		power.Reboot(false)
	},
}
