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
	fp "path/filepath"
	"time"
)

type fileLog struct {
	f    *os.File
	next StackableLogger
}

var _ StackableLogger = (*fileLog)(nil)

var EPrefix = fmt.Errorf("log prefix is unset")

// AddFileLog adds a fileLog to the stack. Existing events are inserted. Name is
// a combination of the prefix (GetPrefix) and the current time, via
// TimestampLayout. See also AddNamedFileLog.
func AddFileLog(dir string) (string, error) {
	prefix := GetPrefix()
	if prefix == "" {
		return "", EPrefix
	}
	err := os.Mkdir(dir, 0755)
	if err != nil && !os.IsExist(err) {
		return "", err
	}
	name := prefix + time.Now().Format(TimestampLayout) + ".log"
	path := fp.Join(dir, name)
	return AddNamedFileLog(path)
}

// AddNamedFileLog adds a fileLog to the stack like AddFileLog, but uses the
// specified name rather than coming up with one.
func AddNamedFileLog(fname string) (string, error) {
	f, err := os.Create(fname)
	if err != nil {
		return "", err
	}
	fl := &fileLog{f: f}
	err = AddLogger(fl, true)
	if err == nil {
		err = SetAttr("Filename", fname)
	}
	if err != nil {
		f.Close()
		os.Remove(fname)
		return "", err
	}
	return fname, nil
}

func (fl *fileLog) AddEntry(e LogEntry) {
	if (e.Flags&flags.NotFile) == 0 && fl.f != nil {
		fmt.Fprintln(fl.f, e.String())
	}
	if fl.next != nil {
		fl.next.AddEntry(e)
	}
}

func (fl *fileLog) ForwardTo(sl StackableLogger) {
	if fl.next == nil || sl == nil {
		fl.next = sl
	} else {
		panic("next already set")
	}
}

const FileLogIdent = "fileLog"

func (fl *fileLog) Ident() string         { return FileLogIdent }
func (fl *fileLog) Next() StackableLogger { return fl.next }

func (fl *fileLog) Finalize() {
	if fl.f != nil {
		err := fl.f.Close()
		if err != nil {
			fmt.Fprintf(os.Stderr, "closing log file: %s", err)
		}
		fl.f = nil
	}
	if fl.next != nil {
		fl.next.Finalize()
	}
}

func LoggingToFile() bool {
	return InStack(FileLogIdent)
}
