// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

//Package power handles poweroff- and reboot-related functionality, including
//running pre-reboot (Preboot) functions registered with the housekeeping pkg.
//
//As a side-effect of import, log.Fatal is set to power.FailReboot.
package power

import (
	"fmt"
	"os"
	"runtime/debug"
	"time"

	"github.com/purecloudlabs/gprovision/pkg/common/rkeep"
	hk "github.com/purecloudlabs/gprovision/pkg/init/housekeeping"
	"github.com/purecloudlabs/gprovision/pkg/log"

	"golang.org/x/sys/unix"
)

// Defines the action taken on failure, which is to reboot. Importing this
// package has the side effect of calling log.SetFatalAction() with this.
var FatalAction = log.FailAction{
	MsgPfx:     "ERROR, rebooting:",
	Terminator: FailReboot,
}

func init() {
	log.SetFatalAction(FatalAction)
}

//Reboot.
func FailReboot() {
	Reboot(false)
}

// Reboot. If network logging is enabled, first notify logServer that the
// current stage succeeded.
func RebootSuccess() {
	StageFinished()
	Reboot(true)
}

// Logs over network that the current stage (mfg, factory restore) has finished.
func StageFinished() {
	msg := log.GetPrefix() + " succeeded, rebooting..."
	rkeep.ReportFinished(msg)
}

//Not for general use - prefer FailReboot() or RebootSuccess()
func Reboot(success bool) {
	/* this func can be called from a defer statement; deferred functions
	   will execute even if panic() was called. exiting or rebooting will
	   mask any such panic, so check for it and log it
	*/
	x := recover()
	if x != nil {
		log.Logf("panic() caught in reboot(success=%t)", success)
		success = false
		log.Msgf("internal error: %s", x)
		stars := "***********************************************************"
		log.Logf("%s\nstack trace:\n%s\n%s", stars, debug.Stack(), stars)
	}

	hk.Preboots.Perform(success)
	if os.Getpid() != 1 {
		fmt.Fprintf(os.Stderr, "pid 1 would reboot here")
		os.Exit(0)
	}
	time.Sleep(2 * time.Second)
	err := unix.Reboot(unix.LINUX_REBOOT_CMD_RESTART)
	if err != nil {
		fmt.Printf("%s", err)
	}
}

func Off() {
	hk.Preboots.Perform(true)
	if os.Getpid() != 1 {
		fmt.Fprintf(os.Stderr, "pid 1 would shutdown here")
		os.Exit(0)
	}
	time.Sleep(2 * time.Second)
	err := unix.Reboot(unix.LINUX_REBOOT_CMD_POWER_OFF)
	if err != nil {
		fmt.Printf("%s", err)
	}
}
