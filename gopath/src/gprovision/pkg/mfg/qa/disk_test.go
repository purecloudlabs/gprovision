// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package qa

import (
	"gprovision/pkg/log/testlog"
	"testing"
)

var testDiskCfgs MainDiskConfigs
var testRecovDisk *Disk

func init() {
	testDiskCfgs = MainDiskConfigs{
		//dev field isn't checked/modified by the functions being tested; here it's used to identify the entry
		{&MainDisk{Model: "mdl", Vendor: "vnd", Size: 12345, Quantity: 1, dev: "1"}},
		{&MainDisk{Model: "mdl2", Vendor: "vnd2", Size: 23456, Quantity: 2, dev: "2"}},
		{&MainDisk{Model: "mdl21", Vendor: "vnd21", Size: 23456, Quantity: 2, dev: "3"}},
		{&MainDisk{Model: "mdl", Vendor: "vnd2", Size: 23456, Quantity: 2, dev: "4"}},
		{&MainDisk{Model: "mdl2", Vendor: "vnd", Size: 23456, Quantity: 2, dev: "5"}},
		{&MainDisk{Model: "mdl3", Vendor: "vnd", Size: 23456, Quantity: 3, dev: "6"}},
		{&MainDisk{Model: "mdl3", Vendor: "vnd", Size: 23456, Quantity: 3, dev: "7a"},
			&MainDisk{Model: "mdl", Vendor: "vnd", Size: 12345, Quantity: 1, dev: "7b"}},
	}
	testRecovDisk = &Disk{Model: "r", Vendor: "v", Size: 32, Quantity: 1, dev: "recovery"}
}

//func populateDisks(cfgs MainDiskConfigs,found MainDisks) (mdisks MainDisks)
func TestPopulateDisks(t *testing.T) {
	populateHelper(t, testDiskCfgs, 0, testDiskCfgs[0], 1, []string{"1"})
	populateHelper(t, testDiskCfgs, 1, testDiskCfgs[1], 1, []string{"2"})
	populateHelper(t, testDiskCfgs, 2, testDiskCfgs[2], 1, []string{"3"})
	populateHelper(t, testDiskCfgs, 3, testDiskCfgs[3], 1, []string{"4"})
	populateHelper(t, testDiskCfgs, 4, testDiskCfgs[4], 1, []string{"5"})
	populateHelper(t, testDiskCfgs, 5, testDiskCfgs[5], 1, []string{"6"})
	populateHelper(t, testDiskCfgs, 6, testDiskCfgs[6], 2, []string{"7a", "7b"})
	populateHelper(t, testDiskCfgs[:5], -1, testDiskCfgs[6], 0, []string{})
}
func populateHelper(t *testing.T, cfgs MainDiskConfigs, wantIdx int, found MainDisks, wantLen int, wantDev []string) {
	present := append([]*Disk{}, testRecovDisk)
	for _, f := range found {
		present = append(present, (*Disk)(f))
	}
	res, ridx := populateDisks(cfgs, present)
	gotLen := len(res)
	if ridx != wantIdx {
		t.Errorf("got index %d, want %d", ridx, wantIdx)
	}
	if gotLen != wantLen {
		t.Errorf("%v: want %d results, got %d", wantDev, wantLen, gotLen)
		t.Logf("results: %v %d", res, ridx)
	} else {
		for _, r := range res {
			found := false
			for _, w := range wantDev {
				if r.dev == w {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("%v: missing %s", wantDev, r.dev)
			}
		}
	}
}

//func cfgMatch(cfg MainDisks, foundDisks []*Disk) bool
func TestCfgMatch(t *testing.T) {
	md := MainDisk{Model: "mdl", Vendor: "vnd", Size: 12345, Quantity: 1, dev: "1"}
	fd := Disk(md)
	recov := Disk{Model: "r", Vendor: "v", Size: 32, Quantity: 1, dev: "recovery"}
	cfg := MainDisks{&md}
	found := Disks{&fd, &recov}
	found2 := Disks{&recov, &fd}

	tlog := testlog.NewTestLog(t, true, false)
	m := cfgMatch(cfg, found)
	if !m {
		t.Errorf("fail ordering 1")
		t.Log(tlog.Buf.String())
	}
	tlog = testlog.NewTestLog(t, true, false)
	m = cfgMatch(cfg, found2)
	if !m {
		t.Errorf("fail ordering 2")
		t.Log(tlog.Buf.String())
	}
	tlog = testlog.NewTestLog(t, true, false)
	found[0].Model += "2"
	m = cfgMatch(cfg, found)
	if m {
		t.Errorf("false match")
		t.Log(tlog.Buf.String())
	}
	//multiple disks, same model,size variation
	//not sure whether this is likely to be encountered in the wild...
	cfg[0].Quantity = 2
	found[0].Model = "mdl"
	fd2 := fd
	fd2.Size += 80
	found = append(found, &fd2)
	tlog = testlog.NewTestLog(t, true, false)
	m = cfgMatch(cfg, found)
	if !m {
		t.Errorf("failed match with differing sizes")
		t.Log(tlog.Buf.String())
	}
}

//func (required MainDiskConfigs) Compare(detected MainDisks, idx int) (errors int)
func TestCompareDiskCfgs(t *testing.T) {
	tstData := []struct {
		detected MainDisks
		i        int
		errs     int
	}{
		{testDiskCfgs[0], 0, 0},
		{testDiskCfgs[0], 1, 2},
		{testDiskCfgs[1], 1, 0},
		{testDiskCfgs[6], 6, 0},
		{testDiskCfgs[6], 5, 1},
		{testDiskCfgs[5], 6, 1},
		{testDiskCfgs[5], -1, 1},
	}
	for i, td := range tstData {
		tlog := testlog.NewTestLog(t, true, false)
		res := testDiskCfgs.Compare(td.detected, td.i)
		if res != td.errs {
			t.Log(tlog.Buf.String())
			t.Errorf("#%d: got %d, want %d", i, res, td.errs)
		}
	}
}
