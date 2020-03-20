// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package log

// This is the default type of log, storing entries in memory and not displaying
// them in any way. See AddConsoleLog, AddFileLog.
type memLog struct {
	entries []LogEntry
	next    StackableLogger
}

var _ StackableLogger = (*memLog)(nil)

// Adds a memLog to the stack; note that this is unlikely to need called as a
// memLog is the default. memLog is the default type of log, storing entries in
// memory and not displaying them in any way.
//
// See also AddConsoleLog, AddFileLog, lcd.AddLcdLog, etc.
func AddMemLog() error { return AddLogger(&memLog{}, false) }

func (ml *memLog) AddEntry(e LogEntry) {
	ml.entries = append(ml.entries, e)
	if ml.next != nil {
		ml.next.AddEntry(e)
	}
}

func (ml *memLog) ForwardTo(sl StackableLogger) {
	if ml.next == nil || sl == nil {
		ml.next = sl
	} else {
		panic("next already set")
	}
}

const MemLogIdent = "memLog"

func (ml *memLog) Ident() string         { return MemLogIdent }
func (ml *memLog) Next() StackableLogger { return ml.next }

func (ml *memLog) Finalize() {
	ml.entries = nil
	if ml.next != nil {
		ml.next.Finalize()
	}
}

//Not part of StackableLogger interface
func (ml *memLog) Entries() []LogEntry { return ml.entries }

// Retrieve all entries logged so far. Requires a memLog in the stack. Probably
// only useful for testing log packages.
func StoredEntries() []LogEntry {
	logStackMtx.Lock()
	defer logStackMtx.Unlock()
	ml := FindInStack(MemLogIdent)
	if ml == nil {
		return nil
	}
	mem := ml.(*memLog)
	return mem.Entries()
}

// Remove a MemLog from the stack. Used once other log(s) have been added to
// prevent accumulation of log entries in memory.
func FlushMemLog() {
	RemoveLogger(MemLogIdent)
}
