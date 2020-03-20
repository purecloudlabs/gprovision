// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package integ

import (
	"gprovision/pkg/common/strs"
	"gprovision/testing/fakeupd"
	"gprovision/testing/vm"
	"os/exec"
	fp "path/filepath"
	"testing"
	"time"
)

func TestLifecycle_Legacy(t *testing.T) { testLifecycle(t, false) }
func TestLifecycle_UEFI(t *testing.T)   { testLifecycle(t, true) }

var bootRecovery string = "Booting 'Recovery'"
var bootNormal string = "Booting 'Normal'"

var forbiddenStrs = []string{"unimplemented", "context deadline exceeded"}

func testLifecycle(t *testing.T, uefi bool) {
	if len(forbiddenStrs) == 0 {
		forbiddenStrs = []string{"segfault"}
	}
	_, mfgKernel := CheckEnv(t)
	defer func() {
		if t.Failed() {
			out, err := exec.Command("df", "-h").CombinedOutput()
			if err != nil {
				t.Logf("getting disk free space: %s", err)
			} else {
				t.Logf("disk free space:\n%s", string(out))
			}
		}
	}()
	//memory
	if M == 0 {
		M = 512
	}
	pfx := "test-gprov-lifecycle"
	// Lifecycle tests typically run in parallel. Unique prefix ensures no
	// races with tempdir cleanup.
	if uefi {
		pfx += "-u"
	} else {
		pfx += "-l"
	}
	//use 1 tmpdir for all stages; under TEMPDIR if set, else default
	tmpdir, cleanup := VmDir(t, pfx, true)
	defer cleanup(t)

	bootk := fp.Join(fp.Dir(mfgKernel), strs.BootKernel())
	infra := vm.MockInfra(t, tmpdir, bootk, true, "", vm.SerNum(uefi), M, 1)
	defer infra.Cleanup()
	qopts := BaselineVM(uefi, M, t, tmpdir)
	qopts.Timeout = 40 * time.Second
	qopts.Devices = append(qopts.Devices,
		vm.ArbitraryKArgs{
			"console=ttyS0",
			"earlyprintk=ttyS0",
			"mfgurl=" + infra.MfgUrl,
			// allow init (and our fake sysd) to log to logServer
			strs.ContinueLoggingEnv() + "=1",
			strs.VerboseEnv() + "=1",
			strs.IntegEnv() + "=lc",
		},
	)
	qopts.Kernel = mfgKernel
	t.Run("mfg", func(t *testing.T) {
		lfile := subtestLogfile(t, qopts, tmpdir)
		q, err := qopts.Start()
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if t.Failed() {
				t.Logf("vm args: %s", q.CmdlineQuoted())
				readOutLfile(t, lfile)
			}
		}()
		expect(t, q,
			"Decompressing Linux...",
			//"Run /init", //not seen in quiet mode
			"mfg mode",
			"valid checksum for stash.txz",
			"writing credentials to",
			"Rebooting to factory restore...",
		)
		vm.Wait(t, q, 10*time.Second)
		if !infra.LSrvr.CheckFinished(vm.SerNum(uefi), strs.MfgLogPfx()) {
			t.Errorf("state is not MfgFinished")
		}
		vm.CheckForbidden(t, infra.LSrvr, uefi, forbiddenStrs)
		vm.CheckFormattingErrs(t, infra.LSrvr, uefi)
	})
	if t.Failed() {
		return
	}
	//mfg stage needs different qemu options than later stages
	PostMfgFixups(qopts)
	t.Run("factory restore", func(t *testing.T) {
		//FIXME factory restore output not going to ttyS0 on uefi
		lfile := subtestLogfile(t, qopts, tmpdir)
		q, err := qopts.Start()
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if t.Failed() {
				t.Logf("vm args: %s", q.CmdlineQuoted())
				readOutLfile(t, lfile)
			}
		}()
		var lines []string
		if uefi {
			lines = []string{
				// with uefi, not possible to differentiate between factory restore
				// and normal boot until after /init starts
				"efi: EFI v2.70 by EDK II",
			}
		} else {
			lines = []string{bootRecovery}
		}
		lines = append(lines,
			"Run /init as init process",
			"mode: recovery",
			"recovery process complete",
			"reboot: machine restart",
		)
		expect(t, q, lines...)
		vm.Wait(t, q, 10*time.Second)
		if !infra.LSrvr.CheckFinished(vm.SerNum(uefi), strs.FRLogPfx()) {
			t.Errorf("state is not FrFinished")
		}
		vm.CheckForbidden(t, infra.LSrvr, uefi, forbiddenStrs)
		vm.CheckFormattingErrs(t, infra.LSrvr, uefi)
	})
	if t.Failed() {
		return
	}
	t.Run("normal boot", func(t *testing.T) {
		lfile := subtestLogfile(t, qopts, tmpdir)
		q, err := qopts.Start()
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if t.Failed() {
				t.Logf("vm args: %s", q.CmdlineQuoted())
				readOutLfile(t, lfile)
			}
		}()
		var lines []string
		if uefi {
			lines = []string{
				// with uefi, not possible to differentiate between factory restore
				// and normal boot until after /init starts
				"efi: EFI v2.70 by EDK II",
			}
		} else {
			lines = []string{bootNormal}
		}
		lines = append(lines,
			"Run /init as init process",
			"mode: normal boot",
		)
		if uefi {
			lines = append(lines, "EXT4-fs (sda1): mounted filesystem")
		} else {
			lines = append(lines, "EXT4-fs (sda2): mounted filesystem")
		}
		lines = append(lines,
			fakeupd.InitRunning,
			fakeupd.Bye,
			"reboot: Power down",
		)
		expect(t, q, lines...)
		vm.Wait(t, q, 10*time.Second)
		vm.CheckForbidden(t, infra.LSrvr, uefi, forbiddenStrs)
		vm.CheckFormattingErrs(t, infra.LSrvr, uefi)
	})
}
