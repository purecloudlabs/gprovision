// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package partitioning

import (
	"fmt"
	"gprovision/pkg/log/testlog"
	"testing"
)

//func (m *mbr) commands() (cmds string)
func TestMBRCommands(t *testing.T) {
	tlog := testlog.NewTestLog(t, true, false)
	p := NewMbr("/dev/null")
	p.Add(99, ESP, true, "a")
	p.Add(0, LinuxRaid, false, "b")
	m := p.(*mbr)
	cmds := m.commands()
	want := ",99M,ef,*\n,,fd,\n"
	got := fmt.Sprintf("%v", cmds)
	if want != got {
		t.Errorf("\nwant %s\ngot  %s", want, got)
	}
	tlog.Freeze()
	l := tlog.Buf.String()
	if l != "" {
		t.Errorf("unexpected log output: %s", l)
	}

	tlog = testlog.NewTestLog(t, true, false)
	p = NewMbr("/dev/null")
	p.Add(99, ESP, true, "c")
	p.Add(0, LinuxRaid, true, "d")
	m = p.(*mbr)
	cmds = m.commands()
	got = fmt.Sprintf("%v", cmds)
	if want != got {
		t.Errorf("\nwant %s\ngot  %s", want, got)
	}
	tlog.Freeze()
	l = tlog.Buf.String()
	if t.Failed() {
		t.Logf("log output: %s", l)
	}

	tlog = testlog.NewTestLog(t, true, false)
	p = NewMbr("/dev/null")
	p.Add(99, ESP, false, "e")
	p.Add(0, LinuxRaid, false, "f")
	m = p.(*mbr)
	cmds = m.commands()
	want = ",99M,ef,\n,,fd,\n"
	got = fmt.Sprintf("%v", cmds)
	if want != got {
		t.Errorf("\nwant %s\ngot  %s", want, got)
	}

	m.committed = true
	tlog.FatalIsNotErr = true
	p.Add(0, Linux, false, "should fail")
	tlog.Freeze()
	if tlog.FatalCount == 1 {
		t.Log("as desired, cannot add partition after commit")
	} else {
		t.Errorf("did not fail to add partition after commit: %#v", m)
	}
	l = tlog.Buf.String()
	if t.Failed() {
		t.Logf("log output: %s", l)
	}

}
