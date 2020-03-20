// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package testing

import (
	"fmt"
	"gprovision/pkg/log"
)

// Copied from testing package, minus the private method that makes it
// unimplementable (why?!)
// Used for easier testing of CheckFormattingErrs, etc.
//
// TB is the interface common to testing.T and testing.B.
type TB interface {
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
	Fail()
	FailNow()
	Failed() bool
	Fatal(args ...interface{})
	Fatalf(format string, args ...interface{})
	Log(args ...interface{})
	Logf(format string, args ...interface{})
	Name() string
	Skip(args ...interface{})
	SkipNow()
	Skipf(format string, args ...interface{})
	Skipped() bool
	Helper()

	// A private method to prevent users implementing the
	// interface and so future additions to it will not
	// violate Go 1 compatibility.
	//
	// DISABLED
	//
	//private()
}

// MockTB mocks testing.TB for test of CheckFormattingErrs. Not perfect -
// not sure how to replicate Fail*/Fatal*/Skip* behavior short of a dynamic
// rewrite of calling code.
type MockTB struct {
	LogCnt, ErrCnt, FatalCnt int
	Skp                      bool
	u                        TB //underlying log or nil
}

var _ TB = (*MockTB)(nil)

func (m *MockTB) Error(va ...interface{}) {
	m.ErrCnt++
	if m.u != nil {
		m.u.Error(va...)
	}
}
func (m *MockTB) Errorf(f string, va ...interface{}) {
	m.ErrCnt++
	if m.u != nil {
		m.u.Errorf(f, va...)
	}
}

func (m *MockTB) Fail() {
	m.FatalCnt++
	if m.u != nil {
		m.u.Fail()
	}
}
func (m *MockTB) FailNow() {
	m.FatalCnt++
	if m.u != nil {
		m.u.FailNow()
	}
}
func (m *MockTB) Fatal(va ...interface{}) {
	m.FatalCnt++
	if m.u != nil {
		m.u.Fatal(va...)
	}
}
func (m *MockTB) Fatalf(f string, va ...interface{}) {
	m.FatalCnt++
	if m.u != nil {
		m.u.Fatalf(f, va...)
	}
}

func (m *MockTB) Log(va ...interface{}) {
	m.LogCnt++
	if m.u != nil {
		m.u.Log(va...)
	}
}
func (m *MockTB) Logf(f string, va ...interface{}) {
	m.LogCnt++
	if m.u != nil {
		m.u.Logf(f, va...)
	}
}

func (m *MockTB) Skip(va ...interface{})            { m.Skp = true }
func (m *MockTB) Skipf(f string, va ...interface{}) { m.Skp = true }
func (m *MockTB) SkipNow()                          { m.Skp = true }

func (m *MockTB) Skipped() bool { return m.Skp }
func (m *MockTB) Failed() bool  { return m.ErrCnt > 0 || m.FatalCnt > 0 }
func (m *MockTB) Name() string  { return "mock" }
func (m *MockTB) Helper()       {}

//Sets underlying log. Not part of TB interface
func (m *MockTB) Underlying(tb TB) {
	m.u = tb
}

type TBLogAdapter struct {
	ContinueOnErr bool
}

var _ TB = (*TBLogAdapter)(nil)

func (tl *TBLogAdapter) Error(va ...interface{}) { tl.Fatalf(fmt.Sprint(va...)) }
func (tl *TBLogAdapter) Errorf(f string, va ...interface{}) {
	if tl.ContinueOnErr {
		log.Logf("ERROR:"+f, va...)
	} else {
		log.Fatalf("ERROR:"+f, va...)
	}
}

func (*TBLogAdapter) Fail()                              { log.Fatalf("Fail() called") }
func (*TBLogAdapter) FailNow()                           { log.Fatalf("FailNow() called") }
func (*TBLogAdapter) Fatal(va ...interface{})            { log.Fatalf(fmt.Sprint(va...)) }
func (*TBLogAdapter) Fatalf(f string, va ...interface{}) { log.Fatalf(f, va...) }

func (*TBLogAdapter) Log(va ...interface{})            { log.Logln(va...) }
func (*TBLogAdapter) Logf(f string, va ...interface{}) { log.Logf(f, va...) }

func (*TBLogAdapter) Skip(va ...interface{})            { log.Log("Skip() called") }
func (*TBLogAdapter) Skipf(f string, va ...interface{}) { log.Log("Skipf() called") }
func (*TBLogAdapter) SkipNow()                          { log.Log("SkipNow() called") }

func (*TBLogAdapter) Skipped() bool { return false }
func (*TBLogAdapter) Failed() bool  { return false }
func (*TBLogAdapter) Name() string  { return "TBLogAdapter" }
func (*TBLogAdapter) Helper()       {}
