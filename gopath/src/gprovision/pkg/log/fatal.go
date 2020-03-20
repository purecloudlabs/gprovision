// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package log

import (
	"gprovision/pkg/log/flags"
	"os"
	"strings"
)

// Type of function called after fatal event has been logged. This could cause
// the unit to pause/hang for some period, power off, reboot, etc.
type FatalFunc func()
type PreFunc func(f string, va ...interface{})

// Actions to take when log.Fatalf() is called. Note that this does not need to
// log the event itself - that's done automatically.
type FailAction struct {
	// Prefix to add to message
	MsgPfx string
	// Pre - like Terminator, but runs before calling log.Finalize() - i.e. log is still writable.
	Pre PreFunc
	// Action to take to exit - reboot, shutdown, exit process, etc. Logs are no
	// longer writable when this is called.
	Terminator FatalFunc
}

var fatalAction = DefaultFatal

// Sets up action to take when fatal event has been logged; see FailAction.
func SetFatalAction(act FailAction) { fatalAction = act }

//Default fatal action is to call os.Exit(1)
var DefaultFatal = FailAction{Terminator: DefaultFatalAction}

func DefaultFatalAction() {
	if strings.HasSuffix(os.Args[0], "test") {
		panic("generic fatal called from test")
	}
	os.Exit(1)
}

// Like Msgf or Logf, but does not return - process will be terminated.
// Behavior modified by SetFatalAction(); see also FailAction struct.
func Fatalf(f string, va ...interface{}) {
	if logStack.Next() == nil && logStack.Ident() == MemLogIdent {
		//save some headscratching if no log sink is configured for the process
		AddConsoleLog(0)
		Log("Fatalf: logging unconfigured")
	}
	FlaggedLogf(flags.Fatal, fatalAction.MsgPfx+f, va...)
	if fatalAction.Pre != nil {
		fatalAction.Pre(fatalAction.MsgPfx+f, va...)
	}
	Finalize()
	fatalAction.Terminator()
}
