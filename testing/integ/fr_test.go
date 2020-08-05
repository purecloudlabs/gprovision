// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package integ

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	fp "path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/u-root/u-root/pkg/qemu"
	"github.com/u-root/u-root/pkg/vmtest"

	"github.com/purecloudlabs/gprovision/build/paths"
	"github.com/purecloudlabs/gprovision/pkg/common/strs"
	"github.com/purecloudlabs/gprovision/pkg/log/testlog"
	"github.com/purecloudlabs/gprovision/testing/fakeupd"
	"github.com/purecloudlabs/gprovision/testing/vm"
)

func TestFactoryRestore(t *testing.T)  { testfr(t, false, false, false) }
func TestFREmergencyImg(t *testing.T)  { testfr(t, true, false, false) }
func TestFREmergencyJson(t *testing.T) { testfr(t, false, true, false) }
func TestFRBadHist(t *testing.T)       { testfr(t, false, false, true) }

var emergencyModeJson = []byte(`{"Preserve":true}`)

func testfr(t *testing.T, eImg, eJson, badHist bool) {
	tlog := testlog.NewTestLog(t, false, false)
	defer tlog.Freeze()

	CheckEnv(t)

	tmpdir, tmpcleanup := VmDir(t, "test-gprov-fr", true)
	defer tmpcleanup(t)

	//memory
	if M == 0 {
		M = 512
	}
	krnl := paths.KNoInitramfs
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
	var ename = strs.EmergPfx() + "testfile" //emergency file name
	if eImg || eJson {
		eUsb := fp.Join(tmpdir, "eusb")
		if err = os.Mkdir(eUsb, 0755); err != nil {
			t.Fatalf("creating emergency usb dir: %s", err)
		}
		var vvfatUsb = qemu.ArbitraryArgs{
			//specify ro _and_ readonly _and_ format=raw to suppress warnings
			"-drive", fmt.Sprintf("file=fat:ro:%s,if=none,id=xhcivv,readonly,format=raw", eUsb),
			"-device", "nec-usb-xhci,id=vvbus",
			"-device", "usb-storage,bus=vvbus.0,drive=xhcivv",
		}
		opts.QEMUOpts.Devices = append(opts.QEMUOpts.Devices,
			vvfatUsb,
			// without real_root, it drops straight into factory restore
			// without checking for emergency files
			vm.ArbitraryKArgs{"real_root=LABEL=anythingwilldo"},
		)
		if eImg {
			err = ioutil.WriteFile(fp.Join(eUsb, ename), upd, 0644)
			if err != nil {
				t.Fatalf("writing emergency update: %s", err)
			}
		} else {
			err = ioutil.WriteFile(fp.Join(eUsb, ename), emergencyModeJson, 0644)
			if err != nil {
				t.Fatalf("writing emergency json: %s", err)
			}
		}
	}

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

	p9 := P9RecovOpts{
		T:          t,
		Qopts:      &opts.QEMUOpts,
		Tmpdir:     tmpdir,
		Upd:        upd,
		Krnl:       krnl,
		Xlog:       infra.LogUrl(),
		BadHistory: badHist,
	}
	p9.Setup()

	//sets qemu output, must be called before qemu starts
	lfile := fp.Join(tmpdir, "serial.log")
	logfile(t, &opts.QEMUOpts, lfile)

	// Create the CPIO and start QEMU. Also returns a cleanup function, but we
	// ignore it and do our own cleanup. Calling it + ours causes panic.
	q, _ := vmtest.QEMUTest(t, opts)

	//how to get output in real time?
	t.Logf("vm starting...")

	defer func() {
		if t.Failed() {
			readOutLfile(t, lfile)
			t.Logf("vm args: %s", q.CmdlineQuoted())
		}
	}()

	vm.RequireTxt(t, q,
		"Decompressing Linux...",
		"Run /init as init process",
	)
	if eImg {
		vm.RequireTxt(t, q,
			"Emergency-mode file(s) found. Checking...",
			"checking "+ename[:len(ename)-4], //last 4 chars are trimmed (normally .upd)
		)
		// requires regex since size could fluctuate
		// "decompressed valid 17M update /ext_usb/pce_emergencytestfile",
		vm.RequireRE(t, q, "decompressed valid [0-9]+M update /ext_usb/"+ename)
	} else if eJson {
		vm.RequireTxt(t, q,
			"Emergency-mode file(s) found. Checking...",
			"Using emergency-mode JSON",
		)
	} else if badHist {
		vm.RequireRE(t, q,
			"history check failed for last or only image .*, applying anyway",
		)
	}
	vm.RequireTxt(t, q,
		"recovery process complete",
		"reboot: machine restart",
	)
	vm.Wait(t, q, 10*time.Second)
	if eJson {
		// Emergency json overrides the json stored on recovery; that file
		// contains remote logging info (necessary to determine state), so
		// we can't test ejson _and_ determine state.
		return
	}
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
