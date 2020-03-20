// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

// Package log is a flexible logging mechanism allowing multiple log sinks,
// outputting to one or more of: the console, a file, an LCD, a networked log
// ingester, etc.
//
// By default, events are retained in memory so they can be re-played into
// new log sinks if/when they are added later on.
package log

import (
	"fmt"
	"gprovision/pkg/log/flags"
	"os"
)

var logPrefix string

// Sets the log prefix, which is used in the file name and other places. Must
// be set before calling AddFileLog()
func SetPrefix(pfx string) {
	logPrefix = pfx
}

// Gets the log prefix
func GetPrefix() string { return logPrefix }

// Msgf is for use with messages suitable for display to the user. Short,
// non-technical. Use must be relatively infrequent, as user will need time
// to read each message or it is useless.
func Msgf(f string, va ...interface{}) { FlaggedLogf(flags.EndUser, f, va...) }

// See Msgf
func Msgln(va ...interface{}) { Msgf(fmt.Sprintln(va...)) }

// See Msgf
func Msg(message string) { Msgf(message) }

// Logf is for use with more technical, or more trivial, messages. Never
// visible to user via lcd. No rate limiting concerns.
func Logf(f string, va ...interface{}) { FlaggedLogf(flags.NA, f, va...) }

// See Logf
func Logln(va ...interface{}) { Logf(fmt.Sprintln(va...)) }

// See Logf
func Log(message string) { Logf(message) }

// If the log stack includes a MemLog, this writes all of its content to stderr.
// no-op otherwise.
func DumpStderr() {
	l := FindInStack(MemLogIdent)
	if l != nil {
		ml := l.(*memLog)
		for _, e := range ml.Entries() {
			fmt.Fprintln(os.Stderr, e.String())
		}
	}
}
