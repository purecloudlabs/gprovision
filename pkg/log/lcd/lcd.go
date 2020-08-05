// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

// Package lcd is a StackableLog that logs events to cfa.DefaultLcd, as long as
// those  events have the EndUserFlag set. This package will not find/init an
// lcd - that must be done separately.
package lcd

import (
	"fmt"

	"github.com/purecloudlabs/gprovision/pkg/hw/cfa"
	"github.com/purecloudlabs/gprovision/pkg/log"
	"github.com/purecloudlabs/gprovision/pkg/log/flags"
)

func AddLcdLog(opts flags.Flag) error {
	if cfa.DefaultLcd == nil {
		return cfa.ENil
	}
	return log.AddLogger(&LcdLog{opts: opts}, false)
}

type LcdLog struct {
	opts flags.Flag
	next log.StackableLogger
}

var _ log.StackableLogger = (*LcdLog)(nil)

func (l *LcdLog) AddEntry(e log.LogEntry) {
	if e.Flags&l.opts != 0 {
		_, _ = cfa.DefaultLcd.Msg(fmt.Sprintf(e.Msg, e.Args...))
	}
	if l.next != nil {
		l.next.AddEntry(e)
	}
}

func (l *LcdLog) ForwardTo(sl log.StackableLogger) {
	if l.next == nil || sl == nil {
		l.next = sl
	} else {
		panic("next already set")
	}
}

const LcdLogIdent = "lcdLog"

func (*LcdLog) Ident() string               { return LcdLogIdent }
func (l *LcdLog) Next() log.StackableLogger { return l.next }

func (l *LcdLog) Finalize() {
	if l.next != nil {
		l.next.Finalize()
	}
}
