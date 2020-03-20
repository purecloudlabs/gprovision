// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package nic

import (
	"gprovision/pkg/log/testlog"
	"strings"
	"testing"
)

//func parseQueueInfo(name string, out []byte) (info nicRssQCfg)
func TestParseQueueInfo(t *testing.T) {
	for _, iface := range queueTestData {
		t.Run(iface.name, func(t *testing.T) {
			tlog := testlog.NewTestLog(t, true, false)
			got := parseQueueInfo(iface.name, iface.out)
			if got != iface.want {
				t.Errorf("%s: got \n%v\nwant\n%v", iface.name, got, iface.want)
			}
			tlog.Freeze()
			l := tlog.Buf.String()
			haveLog := (l != "")
			if haveLog {
				if !iface.expectLogContent {
					t.Errorf("log content: %s", l)
				} else {
					if iface.logContains != "" && !strings.Contains(l, iface.logContains) {
						t.Errorf("expected '%s' in log, did not find it. log content:\n%s", iface.logContains, l)
					}
				}
			}
		})
	}
}

var queueTestData = []struct {
	name             string
	out              []byte
	want             nicQueueCfg
	expectLogContent bool
	logContains      string
}{
	{
		name: "mini-i218",
		/* ---------- mini ----------
		   I was thinking i218 was more powerful (more queues), but from the data below, i210 is

		   lspci | grep Ether
		   00:19.0 Ethernet controller: Intel Corporation Ethernet Connection I218-LM (rev 04)
		   02:00.0 Ethernet controller: Intel Corporation I210 Gigabit Network Connection (rev 03)
		   ---
		   dmesg | grep renamed
		   [    1.055304] e1000e 0000:00:19.0 eno1: renamed from eth0
		   [    1.076276] igb 0000:02:00.0 enp2s0: renamed from eth1
		*/
		// ethtool -l eno1
		out: []byte(`Channel parameters for eno1:
Cannot get device channel parameters
: Operation not supported
`),
		want:             nicQueueCfg{}, // all zeros
		expectLogContent: true,
		logContains:      "appears to have no queues",
	}, {
		name: "mini-i210",
		// ethtool -l enp2s0
		out: []byte(`Channel parameters for enp2s0:
Pre-set maximums:
RX:             0
TX:             0
Other:          1
Combined:       4
Current hardware settings:
RX:             0
TX:             0
Other:          1
Combined:       4
`),
		want: nicQueueCfg{
			rx:       currMax{0, 0},
			tx:       currMax{0, 0},
			other:    currMax{1, 1},
			combined: currMax{4, 4},
		},
	}, {
		name: "original-i350",
		// ethtool -l ens6f3
		out: []byte(`Channel parameters for ens6f3:
Pre-set maximums:
RX:             0
TX:             0
Other:          1
Combined:       8
Current hardware settings:
RX:             0
TX:             0
Other:          1
Combined:       8
`),
		want: nicQueueCfg{
			rx:       currMax{0, 0},
			tx:       currMax{0, 0},
			other:    currMax{1, 1},
			combined: currMax{8, 8},
		},
	}, {
		name: "prototype-tg3",
		// ethtool -l eno1
		out: []byte(`Channel parameters for eno1:
Pre-set maximums:
RX:             4
TX:             4
Other:          0
Combined:       0
Current hardware settings:
RX:             4
TX:             1
Other:          0
Combined:       0
`),
		want: nicQueueCfg{
			rx:       currMax{4, 4},
			tx:       currMax{1, 4},
			other:    currMax{0, 0},
			combined: currMax{0, 0},
		},
	},
}
