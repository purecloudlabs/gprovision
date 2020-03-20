// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package log

import (
	"fmt"
	"gprovision/pkg/log/flags"
	"os"
)

type consoleLog struct {
	flags flags.Flag
	next  StackableLogger
}

// Adds a consoleLog to the stack. Flags determine which events will log to the
// console. Typically this would be flags.NA (everything) or flags.EndUser
// (only messages intended for the end user will be visible)
func AddConsoleLog(flags flags.Flag) {
	_ = AddLogger(&consoleLog{flags: flags}, true)
}

var _ StackableLogger = (*consoleLog)(nil)

func (l *consoleLog) AddEntry(e LogEntry) {
	if l.flags == 0 || e.Flags&l.flags > 0 {
		fmt.Fprintln(os.Stderr, e.String())
	}
	if l.next != nil {
		l.next.AddEntry(e)
	}
}

func (l *consoleLog) ForwardTo(sl StackableLogger) {
	if l.next == nil || sl == nil {
		l.next = sl
	} else {
		panic("next already set")
	}
}

const ConsoleLogIdent = "consoleLog"

func (*consoleLog) Ident() string           { return ConsoleLogIdent }
func (l *consoleLog) Next() StackableLogger { return l.next }

func (l *consoleLog) Finalize() {
	if l.next != nil {
		l.next.Finalize()
	}
}
