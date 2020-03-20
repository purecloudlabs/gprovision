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
	"sync"
	"time"
)

// A type of logger which can be chained/stacked, each adding different
// functionalities. Events can go to a networked log server, to a file, to an
// lcd, or just into memory - and this is transparent to the user.
//
// Note that normal logging should go through non-member functions in this
// package - Logf, Msgf, Fatalf, etc. End users do not need to know the details
// here.
type StackableLogger interface {
	//Add an entry to the log. Must call the same method on the next log in the
	// stack (if not nil).
	AddEntry(e LogEntry)

	// Call to chain one logger to another. It must be an error to call this
	// method on a logger to which another has already been chained.
	ForwardTo(StackableLogger)

	// Returns a string identifying the type of logger, for purposes of ensuring
	// no duplicates in stack.
	Ident() string
	// Returns next StackableLogger or nil
	Next() StackableLogger
	// Finalizes any outstanding entries and releases resources (disconnect
	// network connection, close file, etc). Must call the same method on the
	// next log in the stack (if not nil).
	Finalize()
}

// Top logger on the stack. Note that any functions accessing currentLogger,
// currentLogger.Next(), etc MUST honor the mutex logStackMtx.
var logStack StackableLogger = &memLog{}

// Mutex protecting access to logStack. Must be locked while making changes to
// the stack or adding entries.
var logStackMtx sync.Mutex

type stackErr struct {
	Id string
}

func (se *stackErr) Error() string {
	return fmt.Sprintf("Duplicate logger %s in stack", se.Id)
}

// Flushes data, closes files/connections, etc
func Finalize() {
	logStackMtx.Lock()
	defer logStackMtx.Unlock()
	logStack.Finalize()
}

// Restores the log stack to initial state. Calls Finalize on existing
// logger(s), then replaces the existing stack with a memLog.
func DefaultLogStack() { NewLogStack(&memLog{}) }

//Calls Finalize on existing logger(s), then sets newLog as the topmost logger.
func NewLogStack(newLog StackableLogger) {
	logStackMtx.Lock()
	defer logStackMtx.Unlock()
	if logStack != nil {
		logStack.Finalize()
	}
	logStack = newLog
	ClearAttrs()
}

// Add a logger to the stack. Anything that requires initialization must
// already be initialized. If addPrevious is true, events already logged in
// a MemLog are added to this logger.
//
// End users
//
// End users should prefer the AddXLog() method - AddFileLog(), AddConsoleLog(),
// lcd.AddLcdLog(), etc. AddLogger() is intended to be called by a
// StackableLogger's AddXLog() rather than by end users.
//
// Errors
//
// The only possible error is if the new logger is the same type as an existing
// one.
func AddLogger(sl StackableLogger, addPrevious bool) error {
	logStackMtx.Lock()
	defer logStackMtx.Unlock()
	if addPrevious {
		addPreviousEvents(sl, logStack)
	}
	sl.ForwardTo(logStack)
	err := ForwardFrom(sl, logStack)
	if err == nil {
		logStack = sl
	}
	return err
}

// Verifies that the new logger is not a duplicate of another in the stack.
// Called by AddLogger. Recursive.
func ForwardFrom(newLogger, sl StackableLogger) error {
	if newLogger.Ident() == sl.Ident() {
		return &stackErr{Id: sl.Ident()}
	}
	next := sl.Next()
	if next != nil {
		return ForwardFrom(newLogger, next)
	}
	return nil
}

// Remove a log with the given id from the stack
func RemoveLogger(id string) {
	logStackMtx.Lock()
	defer logStackMtx.Unlock()
	l := logStack
	var prev StackableLogger = nil
	for l != nil {
		next := l.Next()
		if l.Ident() == id {
			l.ForwardTo(nil)
			l.Finalize()
			if prev != nil {
				prev.ForwardTo(next)
			}
			break
		}
		prev = l
		l = next
	}
}

// LogEntry is the primary record type for StackableLogger. As with
// StackableLogger, end users do not use this.
type LogEntry struct {
	Time  time.Time `json:"t"`
	Msg   string
	Args  []interface{} `json:",omitempty"`
	Flags flags.Flag    `json:",omitempty"`
}

// Backend of Logf(), Msgf(), Fatalf(), etc. Translates args to LogEntry's and inserts into topmost log.
func FlaggedLogf(opts flags.Flag, f string, va ...interface{}) {
	logStackMtx.Lock()
	defer logStackMtx.Unlock()
	logStack.AddEntry(LogEntry{
		Time:  time.Now(),
		Flags: opts,
		Msg:   f,
		Args:  va,
	})
}

func (le *LogEntry) String() string {
	var div string
	switch {
	case le.Flags&flags.EndUser != 0:
		div = "-- "
	case le.Flags&flags.Fatal != 0:
		div = "!! "
	case le.Flags == 0:
		div = "*- "
	default:
		div = "?? "
	}
	f := div + le.Time.Format(TimestampLayout) + " " + div + le.Msg
	return fmt.Sprintf(f, le.Args...)
}

// May be called when attaching a new logger, in which case it looks
// for a MemLog in the stack and inserts all its entries into the new log
// before the new log is attached to the stack.
func addPreviousEvents(newlog, current StackableLogger) {
	_, isMem := newlog.(*memLog)
	if isMem {
		//should only be one memLog, so we'd be copying to ourselves
		return
	}
	ml := FindInStack(MemLogIdent)
	if ml != nil {
		mem, ok := ml.(*memLog)
		if ok {
			entries := mem.Entries()
			for _, e := range entries {
				newlog.AddEntry(e)
			}
		}
	}
}

// Return true if a log in the stack matches given id
func InStack(id string) bool {
	return FindInStack(id) != nil
}

// Return StackableLogger matching id, or nil
func FindInStack(id string) StackableLogger {
	l := logStack
	for l != nil {
		if l.Ident() == id {
			return l
		}
		l = l.Next()
	}
	return nil
}
