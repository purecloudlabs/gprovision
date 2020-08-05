// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

// +build !release

// Package testlog hijacks the output of github.com/purecloudlabs/gprovision/pkg/log, and can hijack
// log.Cmd(). By default, this output prints through testing functions but
// it can be stored in a buffer as well - for example, for analysis as part of the
// test.
//
// Cmd() hijacking (via a CmdHijacker function) can be used to ensure that code
// handling conditions not feasibly reproducible locally can be tested.
package testlog

import (
	"bytes"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/purecloudlabs/gprovision/pkg/log"
	"github.com/purecloudlabs/gprovision/pkg/log/flags"
)

//Conforms to log.Logger interface. Constructed via NewTestLog().
type TstLog struct {
	events        leChan
	t             *testing.T    //log here if Buf is nil
	Buf           *bytes.Buffer //if non-nil, Msgf()/Logf() output goes here
	MsgCount      int           //counts number of calls to TstLog.Msgf()
	LogCount      int           //counts number of calls to TstLog.Logf()
	FatalCount    int           //counts number of calls to TstLog.Fatal()
	FatalIsNotErr bool          //if true, do not call t.Errorf() for Fatal()
	freeze        bool          //do not write any more to Buf
	stderr        bool          //also immediately write to stderr
	mu            sync.RWMutex  //still needed, not 1:1 match for mutex in log pkg
	bgWg          sync.WaitGroup
}

//Returns a new TstLog. If bufferLog is true, logging goes to a buffer rather
//than passing directly to t.Log()/t.Error(). Do not share one TstLog between
//tests - create a new one each time.
func NewTestLog(t *testing.T, bufferLog, stderr bool) (tlog *TstLog) {
	tlog = &TstLog{
		events: make(leChan, 1024),
		t:      t,
		stderr: stderr,
	}
	if bufferLog {
		tlog.Buf = new(bytes.Buffer)
	}
	tlog.bgWg.Add(1)
	go tlog.bgProc()
	log.NewLogStack(tlog)
	log.SetFatalAction(log.FailAction{Terminator: func() {}})
	return
}

//Like NewTestLog, but does not use a channel or background thread. Provides more trackable output.
func NewTestLogNoBG(t *testing.T) (tlog *TstLog) {
	tlog = &TstLog{t: t}
	log.TraceHelper(t)
	log.NewLogStack(tlog)
	log.SetFatalAction(log.FailAction{Terminator: func() {}})
	return
}

var _ log.StackableLogger = (*TstLog)(nil)

func (tlog *TstLog) AddEntry(e log.LogEntry) {
	tlog.mu.RLock()
	freeze := tlog.freeze
	tlog.mu.RUnlock()
	if freeze {
		return
	}
	switch e.Flags {
	case flags.EndUser:
		e.Msg = "MSG:" + e.Msg
	case flags.Fatal:
		e.Msg = ">>FATAL()<< " + e.Msg
	case flags.NA:
		e.Msg = "LOG:" + e.Msg
	}
	if tlog.events != nil {
		tlog.events <- e
	} else {
		tlog.t.Helper()
		tlog.handleEvt(e)
	}
}

const TstLogIdent = "tstLog"

func (*TstLog) Ident() string                      { return TstLogIdent }
func (tl *TstLog) Next() log.StackableLogger       { return nil }
func (*TstLog) Finalize()                          {}
func (tl *TstLog) ForwardTo(_ log.StackableLogger) {}

type leChan chan log.LogEntry

//background process started by NewTestLog() but not NewTestLogNoBG()
func (tlog *TstLog) bgProc() {
	tlog.t.Helper()
	defer tlog.bgWg.Done()
	for evt := range tlog.events {
		tlog.handleEvt(evt)
	}
}

func (tlog *TstLog) handleEvt(evt log.LogEntry) {
	tlog.t.Helper()
	f := "@" + evt.Time.Format(stampMilli) + ": " + evt.Msg
	switch evt.Flags {
	case flags.EndUser:
		tlog.MsgCount++
	case flags.Fatal:
		tlog.FatalCount++
		if !tlog.FatalIsNotErr {
			tlog.t.Errorf(f, evt.Args...)
			return
		}
	default:
		tlog.LogCount++
	}
	if tlog.stderr {
		fmt.Fprintf(os.Stderr, f+"\n", evt.Args...)
	}
	if tlog.Buf != nil {
		fmt.Fprintf(tlog.Buf, evt.Msg+"\n", evt.Args...)
	} else {
		tlog.t.Logf(f, evt.Args...)
	}
}

const stampMilli = "15:04:05.000" //time format used for stderr. like time.StampMilli, but leaves off date

//sometimes used in testing to inject separators
func (tlog *TstLog) Logf(f string, va ...interface{}) {
	tlog.t.Helper()
	tlog.AddEntry(log.LogEntry{
		Time: time.Now(),
		Msg:  f,
		Args: va,
	})
}

//call at end of test to sync log and shut down bgProc
func (tlog *TstLog) Freeze() {
	tlog.mu.Lock()
	freeze := tlog.freeze
	tlog.mu.Unlock()
	if freeze {
		return
	}
	log.DefaultLogStack()
	log.SetFatalAction(log.DefaultFatal)

	tlog.mu.Lock()
	defer tlog.mu.Unlock()

	tlog.freeze = true
	if tlog.events == nil {
		return
	}
	for len(tlog.events) > 0 {
		time.Sleep(time.Millisecond)
	}
	close(tlog.events)
	tlog.bgWg.Wait()
}

// just calls testing.T.Errorf
func (tlog *TstLog) TstErrf(f string, va ...interface{}) {
	tlog.t.Helper()
	tlog.t.Errorf(f, va...)
}

//just calls testing.T.Logf
func (tlog *TstLog) TstLogf(f string, va ...interface{}) {
	tlog.t.Helper()
	tlog.t.Logf(f, va...)
}
