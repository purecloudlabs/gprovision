// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package integ

import (
	"gprovision/pkg/log/testlog"
	"gprovision/testing/vm"
	"os"
	"testing"
	"time"

	"github.com/u-root/u-root/pkg/qemu"
)

func TestErase(t *testing.T) {
	tlog := testlog.NewTestLog(t, false, false)
	defer func() {
		tlog.Freeze()
		if t.Failed() {
			t.Logf("output: %s\n", tlog.Buf.String())
		}
	}()
	CheckEnv(t)

	tmpdir, tmpcleanup := VmDir(t, "test-gprov-erase", true)
	defer tmpcleanup(t)

	infra := vm.MockInfra(t, tmpdir, "", false, "", vm.SerNum(false), M, 1)
	defer infra.Cleanup()

	//memory
	if M == 0 {
		M = 512
	}
	qopts := EraseOpts(t, tmpdir)

	// Running the vm modifies the device list; we need original, so create a
	// copy. Needed because it defines the names of the virtual disk that we
	// need to re-use.
	qdevs := make([]qemu.Device, len(qopts.Devices))
	copy(qdevs, qopts.Devices)

	t.Run("prepare", func(t *testing.T) {
		lfile := setupEraseHelper(t, tmpdir, qopts, false)
		q, err := qopts.Start()
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if t.Failed() {
				readOutLfile(t, lfile)
				t.Logf("command:\n%s", q.CmdlineQuoted())
			}
			vm.Wait(t, q, time.Minute)
		}()

		t.Log("running...")
		expect(t, q,
			"patterns written successfully. exiting.",
			"reboot: Power down",
		)
	})
	if t.Failed() {
		t.FailNow()
	}

	t.Run("erase", func(t *testing.T) {
		copy(qopts.Devices, qdevs)
		Setup9pRecov(t, qopts, tmpdir, nil, "", "")
		initrd := Initramfs(tmpdir, "", nil, "initramfs")
		out, err := initrd.Build()
		if err != nil {
			t.Fatal(err)
		}
		qopts.KernelArgs = ""
		qopts.Initramfs = out
		lfile := subtestLogfile(t, qopts, tmpdir)

		q, err := qopts.Start()
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if t.Failed() {
				readOutLfile(t, lfile)
				t.Logf("command:\n%s", q.CmdlineQuoted())
			}
			//use close, not wait - vm does not shut down on erase success
			q.Close()
		}()

		t.Logf("vm starting...")
		milestones := []string{
			"Recovery media at /mnt/recov",
			"Data erase: locating drives...",
			"/dev/sda: pattern overwrite",
			"sda: found pattern in 0 places",
			"Data erase completed successfully.",
		}
		expect(t, q, milestones...)
	})
	if t.Failed() {
		t.FailNow()
	}
	t.Run("verify", func(t *testing.T) {
		qopts.Devices = qdevs
		lfile := setupEraseHelper(t, tmpdir, qopts, true)

		q, err := qopts.Start()
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if t.Failed() {
				readOutLfile(t, lfile)
				t.Logf("command:\n%s", q.CmdlineQuoted())
			}
			vm.Wait(t, q, time.Minute)
		}()

		t.Log("running...")
		expect(t, q,
			"patterns not present - success. exiting.",
			"reboot: Power down",
		)
	})
}

//temporarily override opts, build, run helper binary on disks
func setupEraseHelper(t *testing.T, tmpdir string, qopts *qemu.Options, verify bool) string {
	// erase_integ: avoid unwanted deps in init pkg
	irfs := Initramfs(tmpdir, "", []string{"erase_integ"})
	irfs.Commands[0].Packages = []string{"gprovision/testing/integ/helper/erase"}
	irfs.InitCmd = "erase"
	path, err := irfs.Build()
	if err != nil {
		t.Error(err)
	}
	qopts.Initramfs = path
	qopts.Kernel = os.Getenv("UROOT_KERNEL")
	lfile := subtestLogfile(t, qopts, tmpdir)
	if verify {
		qopts.Devices = append(qopts.Devices, vm.ArbitraryKArgs{"eraseHelper=verify"})
	}
	return lfile
}
