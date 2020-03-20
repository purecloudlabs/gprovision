// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package netexport

import (
	"encoding/json"
	"gprovision/pkg/log/testlog"
	"os"
	"testing"
)

//func (ifaces ifMap) load(cfg io.ReadSeeker) (err error)
func TestLoadIntelAllDHCP(t *testing.T) {
	ifaces := NewIntelMap()
	ifaces.testLoadIntel(t, "data/ps", 7, 0)
}

func TestLoadIntelStaticAndVlans(t *testing.T) {
	ifaces := NewIntelMap()
	ifaces.testLoadIntel(t, "data/staticvlan", 9, 2)
}
func TestLoadIntelChrisCMini(t *testing.T) {
	ifaces := NewIntelMap()
	ifaces.testLoadIntel(t, "data/ccmini", 3, 1)
}

func (ifaces IntelMap) testLoadIntel(t *testing.T, path string, requiredIfaces, requiredVLANs int) {
	cfgf := path + "/Saved_Config.txt"
	cfg, err := os.Open(cfgf)
	if err != nil {
		t.Errorf("%s", err)
	}
	defer cfg.Close()
	tlog := testlog.NewTestLog(t, true, false)

	defer func() {
		tlog.Freeze()
		if tlog.Buf.Len() > 0 {
			t.Log(tlog.Buf.String())
		}
	}()

	err = ifaces.loadIntel(cfg)
	if err != nil {
		t.Errorf("%s", err)
	}
	if len(ifaces) != requiredIfaces {
		for _, i := range ifaces {
			j, e := json.MarshalIndent(i, "", "  ")
			if e != nil {
				t.Error(e)
			}
			t.Logf("%s\n", j)
		}
		t.Errorf("need %d interfaces, got %d", requiredIfaces, len(ifaces))
	}
	var foundVLANs int
	for a, ifaceA := range ifaces {
		for b, ifaceB := range ifaces {
			if a == b {
				continue
			}
			if ifaceA.Mac.String() == ifaceB.Mac.String() {
				if ifaceA.HasVLANChildren == ifaceB.HasVLANChildren || (ifaceA.IsVLAN == false && ifaceB.IsVLAN == false) {
					t.Errorf("%s and %s have same mac", a, b)
				}
				if ifaceA.IsVLAN {
					foundVLANs += 1
				}
			}
		}
		partialStaticCfg := len(ifaceA.IPs) > 0 || len(ifaceA.Routes) > 0 || len(ifaceA.NameServers) > 0
		staticCfg := len(ifaceA.IPs) > 0 && len(ifaceA.Routes) > 0
		if partialStaticCfg && !staticCfg {
			t.Errorf("incomplete static config")
		}
	}
	if foundVLANs != requiredVLANs {
		t.Errorf("want %d vlans, got %d", requiredVLANs, foundVLANs)
	}
}

//func (ifmap ifMap) parseRoutes(routeout []byte) (err error)
func TestParseRoutes(t *testing.T) {
	routes := []byte(`"26",,,"256","3","0.0.0.0/0","10.155.8.1"
"13",,,"256","3","0.0.0.0/0","10.155.8.1"
"14",,,"256","3","::/0","27::1"

`)
	ifaces := NewIfMap()
	err := ifaces.parseRoutes(routes)
	if err == nil {
		t.Errorf("expected error, got nil")
	}
	if len(ifaces) != 0 {
		t.Errorf("expected len 0")
	}
	for _, i := range []int{13, 14, 26} {
		ifaces[i] = &WinNic{}
	}
	err = ifaces.parseRoutes(routes)
	if err != nil {
		t.Errorf("unexpected error %s", err)
	}
	if len(ifaces) != 3 {
		t.Errorf("expected len 3, got %d", len(ifaces))
	}
}
