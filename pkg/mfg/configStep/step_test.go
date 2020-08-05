// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package configStep

import (
	"strings"
	"testing"

	"github.com/purecloudlabs/gprovision/pkg/log"
	"github.com/purecloudlabs/gprovision/pkg/log/testlog"
)

//func (s *step) Run() (err error)
func TestRun(t *testing.T) {
	var s Step
	var err error
	t.Run("Execute", func(t *testing.T) {
		//test that a command will execute - in this case, printing to stdout
		//also tests ESDontCare + success
		tlog := testlog.NewTestLog(t, true, false)
		s = Step{
			Commands: []StepCmd{{Command: `echo -e 'this\040works'`, ExitStatus: ESDontCare}},
			Verbose:  true,
		}
		err = s.Run()
		if err != nil {
			t.Error(err)
		}
		tlog.Freeze()
		l := tlog.Buf.String()
		if !strings.Contains(l, "040works") {
			//Running [echo -e this\040works]...
			t.Error("has input string been changed? needs to include an escape sequence...")
		}
		if !strings.Contains(l, "command output: this works") {
			t.Errorf("unexpected output '%s'", l)
		}
	})
	t.Run("ShouldFail", func(t *testing.T) {
		//will it catch a command that should fail but doesn't?
		s.Commands[0].ExitStatus = ESMustFail
		tlog := testlog.NewTestLog(t, true, false)
		err = s.Run()
		if err == nil {
			tlog.Freeze()
			t.Log(tlog.Buf.String())
			t.Error("must fail")
		}
	})
	t.Run("DoesFail", func(t *testing.T) {
		//will it accept a command that should fail and does?
		s.Commands[0].Command = `false`
		tlog := testlog.NewTestLog(t, true, false)
		err = s.Run()
		tlog.Freeze()
		if err != nil {
			t.Log(tlog.Buf.String())
			t.Error("must succeed, got", err)
		}
	})
	t.Run("ShouldSucceed", func(t *testing.T) {
		//will it catch a command that should succeed but doesn't?
		s.Commands[0].ExitStatus = ESMustSucceed
		tlog := testlog.NewTestLog(t, true, false)
		err = s.Run()
		tlog.Freeze()
		if err == nil {
			t.Log(tlog.Buf.String())
			t.Error("must fail")
		}
	})
	t.Run("FailsDontCare", func(t *testing.T) {
		//will it accept a command that fails when we don't care about the exit status?
		//success + don't care is checked by Execute test
		s.Commands[0].ExitStatus = ESDontCare
		tlog := testlog.NewTestLog(t, true, false)
		err = s.Run()
		tlog.Freeze()
		if err != nil {
			t.Log(tlog.Buf.String())
			t.Error("must succeed, got", err)
		}
	})
}

//func (s *step) applyTmpl(in string) (out string, err error)
func TestApplyTmpl(t *testing.T) {
	tlog := testlog.NewTestLog(t, true, false)
	for _, td := range tmplTestData {
		t.Run(td.name, func(t *testing.T) {
			s := Step{tmplData: StepData{
				CommonData: &CommonTemplateData,
				DLDir:      "dlDir",
			}}
			got, err := s.applyTmpl(td.in)
			if td.expectFailure == (err == nil) {
				if err != nil {
					t.Error(err)
				} else {
					t.Error("expected error, got none")
				}
			}
			if got != td.want {
				t.Errorf("want %s, got %s", td.want, got)
			}
		})
		tlog.Freeze()
		l := tlog.Buf.String()
		if len(l) > 0 {
			t.Log(l)
		}
	}
}

type tmplTestDataS []struct {
	name, in, want string
	expectFailure  bool
}

var tmplTestData tmplTestDataS

func init() {
	log.SetPrefix("test")
	CommonTemplateData.BiosPass = "biosPass"
	CommonTemplateData.IpmiPass = "ipmiPass"
	CommonTemplateData.OSPass = "osPass"
	CommonTemplateData.RecoveryDir = "/recoveryDir"
	CommonTemplateData.Serial = "serial1234"

	tmplTestData = tmplTestDataS{
		{"a", "a", "a", false},
		{"b", "{{.BiosPass}}", CommonTemplateData.BiosPass, false},
		{"c", "{{.IpmiPass}}", CommonTemplateData.IpmiPass, false},
		{"d", "{{.RecoveryDir}}", CommonTemplateData.RecoveryDir, false},
		{"e", "{{.Serial}}", CommonTemplateData.Serial, false},
		{"f", "{{.asdfss}}", "", true},
		{"g", "{{.DLDir}}", "dlDir", false},
		{"h", "{{.OSPass}}", CommonTemplateData.OSPass, false},
	}
}
