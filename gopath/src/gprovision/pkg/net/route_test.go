// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package net

import (
	"fmt"
	"gprovision/pkg/log/testlog"
	"strings"
	"testing"
)

//func (r *route) adjustMetric(newMetric uint64, prevTries int)
//note that we're only testing if this fails, not whether it can succeed
func TestAdjMetric(t *testing.T) {
	tlog := testlog.NewTestLog(t, true, false)

	var r Route
	r.adjustMetric(maxTries+88, 99)
	tlog.Freeze()
	out := tlog.Buf.String()
	if r.Metric != 0 {
		t.Log(out)
		t.Errorf("wrong metric %d", r.Metric)
	}
	if !strings.Contains(out, "too many retries adjusting route metric") {
		t.Log(out)
		t.Errorf("retry logic failure")
	}
	if strings.Contains(out, "incrementing and retrying") {
		t.Log(out)
		t.Errorf("retry logic failure")
	}
	tlog = testlog.NewTestLog(t, true, false)

	r.adjustMetric(3, 0)
	tlog.Freeze()
	out = tlog.Buf.String()
	for i := 1; i <= maxTries; i++ {
		if !strings.Contains(out, fmt.Sprintf("(%d/%d)", i, maxTries)) {
			t.Log(out)
			t.Error("retry error")
		}
	}
	n := strings.Count(out, "failed to add route with metric")
	if n != maxTries {
		t.Log(out)
		t.Errorf("want %d tries, got %d", maxTries, n)
	}
}

//func GetRouteThroughIface(iface, destip string) Route
func TestGetRouteThroughIface(t *testing.T) {
	// ip r g 8.8.8.8 oif enp4s0
	// 8.8.8.8 via 10.254.64.161 dev enp4s0 src 10.254.64.174 uid 1000
	rtail := " via 10.254.64.161 dev enp4s0 src 10.254.64.174"
	uid := " uid 1000"
	tlog := testlog.NewTestLog(t, true, false)

	// instead of executing particular commands, emulate them
	cm := make(testlog.CmdMap)
	rkey := testlog.CmdKey([]string{"ip", "route", "get", "8.8.8.8", "oif", "enp4s0"})
	cm[rkey] = testlog.HijackerData{
		Result: testlog.Result{
			Success: true,
			Res:     "8.8.8.8" + rtail + uid,
		},
		NoRun: true,
	}
	// GetRouteThroughIface should ignore things it doesn't understand in ip's
	// output
	r2key := testlog.CmdKey([]string{"ip", "route", "get", "1.1.1.1", "oif", "enp4s0"})
	cm[r2key] = testlog.HijackerData{
		Result: testlog.Result{
			Success: true,
			Res:     "1.1.1.1 gibberish" + rtail + uid,
		},
		NoRun: true,
	}

	tlog.UseMappedCmdHijacker(cm)
	for _, ip := range []string{"8.8.8.8", "1.1.1.1"} {
		r := GetRouteThroughIface("enp4s0", ip)
		if r.String() != ip+rtail {
			t.Errorf("\nwant %s\n got %s", ip+rtail, r.String())
		}
	}
	tlog.Freeze()
	if t.Failed() {
		t.Log(tlog.Buf.String())
	}
}

// func parseRoute(line string, onlyDefault bool) (bool,Route)
func TestParseRoute(t *testing.T) {
	for _, td := range []struct {
		name, in, out string
	}{
		{
			name: "1",
			in:   "8.8.8.8 via 10.254.64.161 dev enp4s0 src 10.254.64.174 uid 1000",
			//we don't parse or care about uid
			out: "8.8.8.8 via 10.254.64.161 dev enp4s0 src 10.254.64.174",
		}, {
			name: "2",
			in:   "default via 4.3.2.1 dev eth0 proto static",
		}, {
			name: "3",
			in:   "default via 4.3.2.1 dev enp2s0 proto dhcp src 4.3.2.3 metric 1024",
		}, {
			name: "4",
			in:   "default via 4.3.2.1 dev eno1 proto dhcp src 4.3.2.2 metric 1024",
		}, {
			name: "5",
			in:   "default blah via 4.3.2.1 dev eth0 proto static",
			out:  "default via 4.3.2.1 dev eth0 proto static",
		}, {
			name: "6",
			in:   "172.16.0.0/12 via 10.254.64.161 dev enp4s0 proto static metric 1024 onlink",
			out:  "172.16.0.0/12 via 10.254.64.161 dev enp4s0 proto static metric 1024",
		}, {
			name: "7",
			in:   "192.168.133.186 dev tun0 proto kernel scope link src 192.168.133.185",
		},
	} {
		t.Run(td.name, func(t *testing.T) {
			tlog := testlog.NewTestLog(t, true, false)
			success, route := parseRoute(td.in, false)
			if !success {
				t.Error("not success")
				return
			}
			if len(td.out) == 0 {
				td.out = td.in
			}
			s := route.String()
			if s != td.out {
				t.Errorf("mismatch,\n got %s\nwant %s\nr=%#v", s, td.out, route)
			}
			if t.Failed() {
				t.Log(tlog.Buf.String())
			}
		})
	}
}
