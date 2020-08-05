// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package partitioning

import (
	"fmt"
	"strings"
	"testing"

	"github.com/purecloudlabs/gprovision/pkg/log/testlog"
)

//func (g *Gpt) assembleArgs() (args []string)
func TestGPTAssembleArgs(t *testing.T) {
	tlog := testlog.NewTestLog(t, true, false)
	p := NewGpt("/dev/null")
	p.Add(99, ESP, true, "a")
	p.Add(0, LinuxRaid, false, "b")
	g := p.(*gpt)
	a := g.assembleArgs()
	want := `[--new=1::+99M --typecode=1:ef00 --change-name=1:a --new=2:: --typecode=2:fd00 --change-name=2:b]`
	got := fmt.Sprintf("%v", a)
	if want != got {
		t.Errorf("\nwant %s\ngot  %s", want, got)
	}
	tlog.Freeze()
	l := tlog.Buf.String()
	if l != "" {
		t.Errorf("unexpected log output: %s", l)
	}

	tlog = testlog.NewTestLog(t, true, false)
	g.partitions[0].boot = false
	a = g.assembleArgs()
	got = fmt.Sprintf("%v", a)
	if want != got {
		t.Errorf("\nwant %s\ngot  %s", want, got)
	}
	tlog.Freeze()
	l = tlog.Buf.String()
	if !strings.Contains(l, "WARNING: UEFI always only boots ESP partitions") {
		t.Errorf("expected warning in log, got %q", l)
	}

	tlog = testlog.NewTestLog(t, true, false)
	g.partitions[0].boot = true
	g.partitions[1].boot = true
	a = g.assembleArgs()
	got = fmt.Sprintf("%v", a)
	if want != got {
		t.Errorf("\nwant %s\ngot  %s", want, got)
	}
	tlog.Freeze()
	l = tlog.Buf.String()
	if !strings.Contains(l, "WARNING: UEFI always only boots ESP partitions") {
		t.Errorf("expected warning in log, got %s", l)
	}
	tlog = testlog.NewTestLog(t, true, false)
	g.committed = true
	tlog.FatalIsNotErr = true
	p.Add(0, Linux, false, "should fail")
	tlog.Freeze()
	if tlog.FatalCount == 1 {
		t.Log("as desired, cannot add partition after commit")
	} else {
		t.Errorf("did not fail to add partition after commit: %#v", g)
	}
}
