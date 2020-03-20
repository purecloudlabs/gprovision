// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package integ

import (
	"gprovision/pkg/common/strs"
	"gprovision/pkg/log"
	gtst "gprovision/testing"
	"gprovision/testing/util"
	"gprovision/testing/vm"
	"os"
	"os/exec"
	fp "path/filepath"

	uq "github.com/u-root/u-root/pkg/qemu"
)

type KBuildOpts struct {
	Recipe, Mfg, Norm, Test bool
}
type UserOpts struct {
	Update, CleanTmp bool
	Kvm, Edit        bool
	Cpus             int
	Kernels          KBuildOpts
	Qemu             *uq.Options
	Mtb              *gtst.MockTB
	Tmpdir           string
	Infra            *vm.Mockinfra
	Cleanup          func()
	TmpPfx           string
}

// setup/boilerplate for mfgFrNorm
func (uo *UserOpts) VmSetup() {
	if uo.Kernels.Recipe == uo.Kernels.Mfg {
		log.Fatalf("need exactly one of recipe, mfg")
	}
	//(update, cleanTmp, kvm bool, cpus int) (*uq.Options, *gtst.MockTB, string, *vm.Mockinfra, func()) {
	Keep = true //cleanup shouldn't delete dir even on success

	if uo.Update {
		var args []string
		if uo.Kernels.Recipe {
			args = append(args, "kernel:recipe")
		}
		if uo.Kernels.Mfg {
			args = append(args, "kernel:linuxmfg")
		}
		if uo.Kernels.Norm {
			args = append(args, "kernel:boot")
		}
		if uo.Kernels.Test {
			args = append(args, "kernel:testkernel")
		}
		m := exec.Command("mage", args...)
		m.Stderr = os.Stderr
		m.Stdout = os.Stdout
		err := m.Run()
		if err != nil {
			log.Fatalf("running mage: %s", err)
		}
	} else {
		log.Log("not updating kernels (!), -u not passed")
	}
	if Tmp != "" {
		os.Setenv("TEMPDIR", Tmp)
	}
	uo.Mtb = &gtst.MockTB{}
	uo.Mtb.Underlying(&gtst.TBLogAdapter{ContinueOnErr: true})

	var integCleanup func(gtst.TB)
	uo.Tmpdir, integCleanup = VmDir(uo.Mtb, uo.TmpPfx, uo.CleanTmp)

	if len(KDir) == 0 {
		rr, err := util.RepoRoot()
		if err != nil {
			log.Fatalf("repo root: %s", err)
		}
		KDir = fp.Join(rr, "work")
	}
	var normKernel string
	if uo.Kernels.Norm {
		normKernel = fp.Join(KDir, strs.BootKernel())
		if _, err := os.Stat(normKernel); err != nil {
			log.Fatalf("missing kernel, check -kdir: %s", err)
		}
	}
	if len(Img) > 0 && M < 5120 {
		M = 5120
		log.Logf("bumping memory to %dM", M)
	}
	if M == 0 {
		M = 512
	}

	uo.Infra = vm.MockInfra(uo.Mtb, uo.Tmpdir, normKernel, false, Img, vm.SerNum(Uefi), M, uo.Cpus)

	//set env vars u-root's testvm package expects
	EnvFromSys(uo.Cpus, uo.Kvm)

	uo.Qemu = BaselineVM(Uefi, M, uo.Mtb, uo.Tmpdir)
	uo.Qemu.Devices = append(uo.Qemu.Devices,
		vm.ArbitraryKArgs{
			"console=ttyS0",
			"earlyprintk=ttyS0",
			strs.ContinueLoggingEnv() + "=1",
			strs.VerboseEnv() + "=1",
		},
	)
	if P9 {
		uo.Qemu.Devices = append(uo.Qemu.Devices,
			uq.P9Directory{Dir: uo.Tmpdir},
			vm.ArbitraryKArgs{strs.IntegEnv() + "=lc"},
		)
	} else {
		uo.Qemu.Devices = append(uo.Qemu.Devices,
			vm.ArbitraryKArgs{strs.LogEnv() + "=" + uo.Infra.LogUrl()},
		)
	}
	if KOverride {
		uo.Qemu.Devices = append(uo.Qemu.Devices,
			vm.ArbitraryKArgs{"KERNEL_OVERRIDE=recovery"},
		)
	}
	if Lcd {
		dev, err := GetLcd()
		if err != nil {
			log.Fatalf("lcd: %s", err)
		}
		uo.Qemu.Devices = append(uo.Qemu.Devices, dev)
	}
	var installerKrnl, mfgKernel string
	if uo.Kernels.Recipe {
		installerKrnl = fp.Join(KDir, "installer.efi")
		if _, err := os.Stat(installerKrnl); err != nil {
			log.Fatalf("missing kernel, check -kdir: %s", err)
		}
		uo.Qemu.Kernel = installerKrnl
	} else if uo.Kernels.Mfg {
		mfgKernel = fp.Join(KDir, strs.MfgKernel())
		if _, err := os.Stat(mfgKernel); err != nil {
			log.Fatalf("missing kernel, check -kdir: %s", err)
		}
		uo.Qemu.Kernel = mfgKernel
		uo.Qemu.Devices = append(uo.Qemu.Devices,
			vm.ArbitraryKArgs{"mfgurl=" + uo.Infra.MfgUrl},
		)
	}

	uo.Qemu.SerialOutput = &NoEscape{Out: os.Stdout}

	uo.Cleanup = func() {
		integCleanup(uo.Mtb)
		uo.Infra.Cleanup()
	}
}
