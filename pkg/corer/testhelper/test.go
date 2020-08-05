// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

// +build !release

//Package testhelper does setup for tests requiring a core dump.
package testhelper

import (
	"io/ioutil"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

//setup for tests requiring a core dump
func CoreHelper(t *testing.T) (dumpFile, testExe string) {
	f, err := ioutil.TempFile("", "gotest*.core")
	if err != nil {
		t.Skip(err)
	}
	f.Close()
	dumpFile = f.Name() //"dump.core"
	testExe = "testexe" //put in working dir - it's often not possible to execute files in /tmp
	err = exec.Command("gdb", "--version").Run()
	if err != nil {
		t.Skip("gdb not found")
	}
	err = exec.Command("coredumpctl", "-h").Run()
	if err != nil {
		/*
			it'd be possible to support some other mechanisms, but it's
			unlikely this test will need to run on those systems
		*/
		t.Skip("unable to get cores on this system")
	}
	sleep, err := exec.LookPath("sleep")
	if err != nil {
		t.Skip("unable to find suitable binary on host system")
	}
	slp, err := ioutil.ReadFile(sleep)
	if err == nil {
		err = ioutil.WriteFile(testExe, slp, 0700)
	}
	if err != nil {
		t.Skip("unable to copy binary:", err)
	}
	defer os.Remove(testExe)
	tst := exec.Command("./"+testExe, "10")
	if err := tst.Start(); err != nil {
		t.Error(err)
	}
	time.Sleep(time.Second / 10)
	if err := tst.Process.Signal(syscall.SIGSEGV); err != nil {
		t.Error(err)
	}
	err = tst.Wait()
	if err == nil {
		t.Skip("should exit with error")
	}
	if tst.ProcessState.Success() {
		t.Skip("process not failing as expected")
	}
	dump := exec.Command("coredumpctl", "dump", testExe)
	f, err = os.Create(dumpFile)
	if err != nil {
		t.Skip(err)
	}
	dump.Stdout = f
	err = dump.Run()
	f.Close()
	if err != nil {
		t.Skip(err)
	}
	return
}
