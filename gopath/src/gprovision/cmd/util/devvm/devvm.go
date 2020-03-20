// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

// Cmd devvm re-uses integ test logic to spin up VMs for dev work. Since it's
// for dev, there are no timeouts unlike integ.
//
// go run gprovision/pkg/testing/integ/run/devvm.go -img /path/to/PRODUCT.Os.Plat.2019-12-19.8704.upd -tmp /var/tmp -lcd
package main

import (
	"flag"
	"gprovision/pkg/common/rlog"
	"gprovision/pkg/log"
	gtst "gprovision/testing"
	"gprovision/testing/integ"
	"gprovision/testing/vm"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func main() {
	//unless we do this, lots of test flags are present because integ pulls in testing pkg
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	integ.Flags()
	update := flag.Bool("u", false, "update mfg and boot kernels using mage")
	cleanTmp := flag.Bool("c", true, "clean up old tmp dirs found in -tmp. Ignored with -norm.")
	cpus := flag.Int("smp", 1, "number of CPUs for vm")
	kvm := flag.Bool("kvm", true, "enable kvm hw accel")
	normInfra := flag.String("norm", "", "start infra and do normal boot on existing disks in given dir")
	edit := flag.Bool("edit", false, "drop to shell before first normal boot")

	flag.Parse()

	log.AddConsoleLog(0)
	log.FlushMemLog()
	log.AdaptStdlog(nil, 0, true)

	if len(*normInfra) > 0 {
		normal(*normInfra, *update)
		return
	}
	uo := &integ.UserOpts{
		Update:   *update,
		CleanTmp: *cleanTmp,
		TmpPfx:   "devvm",
		Kvm:      *kvm,
		Cpus:     *cpus,
		Edit:     *edit,
		Kernels: integ.KBuildOpts{
			Mfg:  true,
			Norm: true,
		},
	}

	mfgFrNorm(uo)
}

// Manufacture, factory restore, and normal boot. Last in window.
func mfgFrNorm(uo *integ.UserOpts) {
	start := time.Now()
	uo.VmSetup()
	log.Logf("tempdir %s", uo.Tmpdir)
	defer uo.Cleanup()

	//mfg
	v, err := uo.Qemu.Start()
	if err != nil {
		log.Fatalf("starting qemu (mfg): %s", err)
	}
	vm.Wait(uo.Mtb, v, 2*time.Minute)
	if uo.Mtb.Failed() {
		log.Logf("tmpdir=%s", uo.Tmpdir)
		log.Fatalf("waiting for qemu (mfg): error")
	}

	mfgEnd := time.Now()
	log.Logf("mfg done, took %s", mfgEnd.Sub(start))

	mfgCmd, _ := uo.Qemu.Cmdline() //used for editing fs below

	integ.PostMfgFixups(uo.Qemu)

	//factory restore
	v, err = uo.Qemu.Start()
	if err != nil {
		log.Fatalf("starting qemu (fr): %s", err)
	}
	vm.Wait(uo.Mtb, v, 2*time.Minute)
	if uo.Mtb.Failed() {
		log.Fatalf("waiting for qemu (fr): timeout")
	}

	log.Logf("fr done, took %s", time.Since(mfgEnd))

	//optionally, start a shell allowing file systems to be edited
	if uo.Edit {
		editFromMfg(mfgCmd)
	}

	//normal boot

	//print info that may be useful while vm is running.
	log.Logf("temp dir %s", uo.Tmpdir)
	elems := strings.Split(uo.Infra.TmplData.LAddr, ":")
	var port int
	if len(elems) > 3 || len(elems) < 2 {
		log.Fatalf("unable to parse LAddr %s", uo.Infra.TmplData.LAddr)
	}
	if len(elems) > 2 {
		port, err = strconv.Atoi(elems[2])
	} else {
		port, err = strconv.Atoi(elems[1])
	}
	if err != nil {
		log.Fatalf(err.Error())
	}
	log.Logf("logserver http://localhost:%s/view/%s", port, vm.SerNum(integ.Uefi))

	//remove nographic, add usb tablet to prevent mouse grab
	q := integ.WindowedVMCmd(uo.Qemu)

	integ.WriteJson(uo.Tmpdir, q, port)

	//run it
	err = q.Run()
	if err != nil {
		log.Logf("error %s", err)
	}
	log.Logf("to run again:\n%s", integ.ExecQuoted(q.Args))
	log.Logf("note, mock infra (mfg fileserver, logging) will not be present if ran again")
}

//given a qemu command to run mfg, this derives a command that will drop to a shell
//uses a window
func editFromMfg(mfgCmd []string) {
	var cmd []string
	kargIdx := 0
	for i := 0; i < len(mfgCmd); i++ {
		if mfgCmd[i] == "-append" {
			//locate args given to the kernel
			//this value is next arg, regardless of whether we've encountered nographic yet
			kargIdx = len(cmd) + 1
		}
		if mfgCmd[i] != "-nographic" {
			cmd = append(cmd, mfgCmd[i])
		}
	}
	kargs := cmd[kargIdx]
	kargs = strings.Join([]string{kargs, "SHELL=shell"}, " ")
	kargs = strings.Replace(kargs, "console=ttyS0", "", -1)
	cmd[kargIdx] = kargs

	log.Logf("edit command: %v", cmd)

	e := exec.Command(cmd[0])
	e.Args = cmd
	e.Stdin = os.Stdin
	e.Stdout = os.Stdout
	e.Stderr = os.Stderr
	err := e.Run()
	if err != nil {
		log.Logf("editing: %s", err)
	}
}

//normal boot only, using files in existing dir
func normal(vmdir string, update bool) {
	args, port := integ.ReadJson(vmdir)

	//start log server on same port as before
	mtb := &gtst.MockTB{}
	mtb.Underlying(&gtst.TBLogAdapter{ContinueOnErr: true})
	rlog.MockServerAt(mtb, vmdir, ":"+strconv.Itoa(port))

	if update {
		panic("unimpl")
		//TODO
		//rebuild initramfs
		//add args to run vm with test kernel + initramfs
	}

	//start vm
	vm := exec.Command(args[0])
	vm.Args = args
	vm.Stdout = os.Stdout
	vm.Stderr = os.Stderr

	err := vm.Run()
	if err != nil {
		log.Fatalf("vm run: %s", err)
	}
}
