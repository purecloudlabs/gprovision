// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package integ

import (
	"fmt"
	"gprovision/pkg/common/strs"
	"gprovision/pkg/log/testlog"
	"gprovision/testing/vm"
	fp "path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/u-root/u-root/pkg/vmtest"
)

//unlike the other tests here, not an integ test. verifies functionality of getLcd()
func TestGetLcd(t *testing.T) {
	if Lcd == false {
		t.Skipf("-lcd unset. plug in a crystalfontz lcd and re-run with -lcd.")
	}
	dev, err := GetLcd()
	if err != nil {
		t.Error(err)
	}
	if dev != nil {
		t.Log(dev.Cmdline())
	}
}

//output by script in vm pkg, see stash_sh
var stasherMilestones = []string{"stash command executing..."}

// TestMfg runs an init exercising mfg code inside a vm.
func TestMfg(t *testing.T) {
	tlog := testlog.NewTestLog(t, false, false)
	defer tlog.Freeze()

	CheckEnv(t)

	tmpdir, tmpcleanup := VmDir(t, "test-gprov-mfg", true)
	defer tmpcleanup(t)

	//memory
	if M == 0 {
		M = 512
	}
	infra := vm.MockInfra(t, tmpdir, "", false, "", vm.SerNum(false), M, 1)
	defer infra.Cleanup()

	obuf, multi := vm.CopyOutput(t)
	opts, err := Mfgopts(t, tmpdir, infra.MfgUrl, multi)
	if err != nil {
		t.Fatal(err)
	}

	// Create the CPIO and start QEMU.
	q, cleanup := vmtest.QEMUTest(t, opts)

	//how to get output in real time?
	defer cleanup()
	t.Logf("vm starting...")
	milestones := []string{
		"Run /init as init process",
		"device matches QEMU-mfg-test - GPROV_QEMU:mfg_test:Not Specified ",
		"Detected disks match MainDiskConfigs\\[2\\]",
		"hw validation: no problems",
		"formatting /dev/sdb1 as ext3, label " + strs.RecVolName(),
		"downloading " + strs.ImgPrefix() + ".*upd",
		"valid checksum for " + strs.ImgPrefix() + ".*upd",

		//boot kernel would download here

		//first of ConfigSteps defined in json
		"command output: sample command executing...",
	}
	milestones = append(milestones, stasherMilestones...)
	logPfx := strs.MfgKernel()
	logPfx = strings.Trim(logPfx, fp.Ext(logPfx))
	creds := infra.LSrvr.MockCreds("QEMU01234")
	milestones = append(milestones,
		//2nd  ConfigStep defined in json
		"Template expansion in Another step: ls -lR {{ .RecoveryDir }}",
		logPfx+"20[0-9]*_[0-9]*.log", //this is the log being written to disk
		"command output: serial=QEMU01234",
		fmt.Sprintf("command output: %s %s %s", creds.OS, creds.BIOS, creds.IPMI),
		"image verified",
		"Rebooting to factory restore...",
	)
	for _, m := range milestones {
		if _, err := q.ExpectRE(regexp.MustCompile(m)); err != nil {
			t.Fatalf("error '%s' while waiting for\n%s", err, m)
		}
	}
	forbiddenStrings := []string{
		"impl unset",
		"logging to server failed",
		"request canceled",
		"SECRETSECRETSECRET",
	}
	if !t.Failed() {
		for _, s := range forbiddenStrings {
			if strings.Contains(obuf.String(), s) {
				t.Errorf("output contains forbidden string %s", s)
			}
		}
	}
}
