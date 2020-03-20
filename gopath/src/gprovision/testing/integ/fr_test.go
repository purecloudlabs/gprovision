// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package integ

import (
	"gprovision/pkg/common/strs"
	"gprovision/pkg/log/testlog"
	"gprovision/testing/fakeupd"
	"gprovision/testing/vm"
	"io/ioutil"
	"net/http"
	"os"
	fp "path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/u-root/u-root/pkg/vmtest"
)

func TestFactoryRestore(t *testing.T) {
	tlog := testlog.NewTestLog(t, false, false)
	defer tlog.Freeze()

	CheckEnv(t)

	tmpdir, tmpcleanup := VmDir(t, "test-gprov-fr", true)
	defer tmpcleanup(t)

	//memory
	if M == 0 {
		M = 512
	}
	krnl := fp.Join(os.Getenv("INFRA_ROOT"), "work/test_kernel")
	upd, err := fakeupd.Make(tmpdir, krnl, "")
	if err != nil {
		t.Fatal(err)
	}
	infra := vm.MockInfra(t, tmpdir, "", false, "", vm.SerNum(false), M, 1)
	defer infra.Cleanup()

	opts := FRopts(t, tmpdir)
	opts.QEMUOpts.Kernel = krnl
	opts.QEMUOpts.Devices = append(opts.QEMUOpts.Devices, vm.ArbitraryKArgs{
		strs.VerboseEnv() + "=1",
	})

	opts.QEMUOpts.Timeout = 200 * time.Second

	//sanity check that the server is up
	host := strings.Replace(infra.LogUrl(), "10.0.2.2", "localhost", -1)
	if !strings.HasPrefix(host, "http://") {
		host = "http://" + host
	}
	host = strings.TrimSuffix(host, "/")
	resp, err := http.Get(host + "/recent/")
	if err != nil {
		t.Fatalf("recent: %s", err)
	}
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("recent: %s", err)
	}
	if !strings.Contains(string(data), "Recent Activity") {
		t.Errorf("/recent page not as expected: %s", string(data))
		t.Logf("host %s", host)
		t.FailNow()
	}
	resp.Body.Close()

	Setup9pRecov(t, &opts.QEMUOpts, tmpdir, upd, krnl, infra.LogUrl())

	//sets qemu output, must be called before qemu starts
	lfile := fp.Join(tmpdir, "serial.log")
	logfile(t, &opts.QEMUOpts, lfile)

	// Create the CPIO and start QEMU. Also returns a cleanup function, but we
	// ignore it and do our own cleanup. Calling it + our causes panic.
	q, _ := vmtest.QEMUTest(t, opts)

	//how to get output in real time?
	t.Logf("vm starting...")

	defer func() {
		if t.Failed() {
			readOutLfile(t, lfile)
			t.Logf("vm args: %s", q.CmdlineQuoted())
		}
	}()

	milestones := []string{
		"Decompressing Linux...",
		"Run /init as init process",
		"recovery process complete",
		"reboot: machine restart",
	}
	expect(t, q, milestones...)
	vm.Wait(t, q, 10*time.Second)
	if !infra.LSrvr.CheckFinished(vm.SerNum(false), strs.FRLogPfx()) {
		t.Errorf("state is not FrFinished")
	}

	forbiddenStrs := []string{
		"logging to server failed",
		"request canceled",
		"impl unset",
		"context deadline exceeded",
	}
	vm.CheckForbidden(t, infra.LSrvr, false, forbiddenStrs)

	ids := infra.LSrvr.Ids()
	if len(ids) != 1 {
		t.Errorf("expect 1 id from LSrvr, got %d: %v", len(ids), ids)
	}
}
