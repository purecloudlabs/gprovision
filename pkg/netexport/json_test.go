// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package netexport

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/purecloudlabs/gprovision/pkg/log/testlog"
)

func TestJson(t *testing.T) {
	ifmap := NewIntelMap()
	cfg, err := os.Open("data/staticvlan/Saved_Config.txt")
	if err != nil {
		t.Errorf("%s", err)
	}
	err = ifmap.loadIntel(cfg)
	if err != nil {
		t.Errorf("%s", err)
	}
	data, err := ifmap.toJson()
	if err != nil {
		t.Errorf("%s", err)
	}
	cnt := strings.Count(string(data), "WinName")
	if cnt != 9 {
		t.Errorf("need 9, got %d", cnt)
	}
	if len(data) < 1500 {
		t.Logf("%s", string(data))
		t.Errorf("data too short: %d", len(data))
	}
}
func (i IntelMap) toJson() (encoded []byte, err error) {
	encoded, err = json.Marshal(i)
	return

}

//count number of interfaces
func ifCount(t *testing.T) int {
	out, err := exec.Command("ip", "-brief", "link").CombinedOutput()
	if err != nil {
		t.Errorf("error %s getting list of nics\nout: %s\n", err, out)
	}
	lines := strings.Split(string(out), "\n")
	n := len(lines)
	for _, l := range lines {
		if strings.Contains(l, "LOOPBACK") || strings.TrimSpace(l) == "" {
			n--
		}
	}
	return n
}
func TestRouteList(t *testing.T) {
	tlog := testlog.NewTestLog(t, true, false)

	defer func() {
		tlog.Freeze()
		if t.Failed() && tlog.Buf.Len() > 0 {
			t.Log(tlog.Buf.String())
		}
	}()

	ifmap := NewIfMap()
	err := ifmap.GetAddrs()
	if !strings.Contains(err.Error(), "powershell.exe\": executable file not found") {
		t.Error(err)
	}
	need := ifCount(t)
	if len(ifmap) != need {
		t.Errorf("need %d interfaces, got %d", need, len(ifmap))
	}
	data, err := ifmap.ToJson()
	if err != nil {
		t.Errorf("json err: %s", err)
	}
	if t.Failed() {
		t.Logf("%s\n\n", data)
	}
}
