// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package testlog

import (
	"gprovision/pkg/log"
	"os/exec"
	"testing"
)

//func (tlog *tstLog) UseMappedCmdHijacker(m ArgMap) CmdHijacker
func TestUseMappedCmdHijacker(t *testing.T) {
	m := make(CmdMap)
	tlog := NewTestLog(t, true, false)
	tlog.UseMappedCmdHijacker(m)
	tru := exec.Command("true")
	res, success := log.Cmd(tru)
	if !success {
		t.Log(res)
		t.Log(tlog.Buf.String())
		t.Errorf("failed")
	}
	if len(m) != 1 {
		t.Log(tlog.Buf.String())
		t.Errorf("bad len - %#v", m)
	}
	if m[CmdKey(tru.Args)].RunCount != 1 {
		t.Log(tlog.Buf.String())
		t.Errorf("bad count - %#v", m)
	}

	u := exec.Command("sha1sum", "/dev/null")
	ukey := CmdKey(u.Args)
	b := exec.Command("badcommand", "blah", "blah")
	b2 := exec.Command("badcommand", "blah", "blah", "blah")
	b3 := b //copy before it runs, cannot copy/reuse after

	bkey := CmdKey(b.Args)
	m[bkey] = HijackerData{
		Result: Result{Success: true, Res: "fake output"},
		NoRun:  true,
	}
	tlog.Buf.Truncate(0)
	res, success = log.Cmd(u)
	if !success || res != "da39a3ee5e6b4b0d3255bfef95601890afd80709  /dev/null\n" {
		t.Log(tlog.Buf.String())
		t.Errorf("%#v: bad result %t %s", u, success, res)
	}
	if res != m[ukey].Result.Res || success != m[ukey].Result.Success {
		t.Log(tlog.Buf.String())
		t.Errorf("%v: mismatch between returned and stored values %s %s %t %t", u.Args, res, m[ukey].Result.Res, success, m[ukey].Result.Success)
	}
	tlog.Buf.Truncate(0)
	res, success = log.Cmd(b)
	if !success || res != "fake output" {
		t.Log(tlog.Buf.String())
		t.Errorf("%v: returning stored result failed - %t %s", b.Args, success, res)
	}
	tlog.Buf.Truncate(0)
	res, success = log.Cmd(b2)
	if success {
		t.Log(tlog.Buf.String())
		t.Errorf("should fail")
	}
	if res != "" {
		t.Log(tlog.Buf.String())
		t.Errorf("unexpected output %s", res)
	}
	tlog.Buf.Truncate(0)
	log.Cmd(b3)
	count := m[bkey].RunCount
	if count != 2 {
		t.Log(tlog.Buf.String())
		t.Errorf("want 2 runs, got %d", count)
	}
}
