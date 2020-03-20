// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package status

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	fp "path/filepath"
	"testing"
	"time"
)

var sysd bool

func TestMain(m *testing.M) {
	sysd = IsSystemd()
	if sysd {
		setup()
	} else {
		fmt.Printf("systemd is not running on this system - cannot test this package\n")
		//we still let the tests run, as they'll print out messages about being skipped
	}
	rc := m.Run()
	if sysd {
		teardown()
	}
	os.Exit(rc)
}

//func IsActive(service string) bool
func TestIsActive(t *testing.T) {
	if !IsSystemd() {
		t.Skipf("Not running systemd")
	}
	if !IsActive("init.scope") {
		t.Errorf("init.scope isn't active?!")
	}
	if IsActive("g1bb3r15h.invalid") {
		t.Errorf("nonsense service 'g1bb3r15h.invalid' is active?!")
	}

	//test against services we create
	ctx := UserContext()
	if ctx.IsActive("goTestTrue") {
		t.Errorf("service should not be active")
	}
	if ctx.IsActive("goTestFalse") {
		t.Errorf("service should not be active")
	}

	if ctx.Start("goTestTrue") != nil {
		t.Errorf("service failed to start")
	}
	if ctx.Start("goTestFalse") != nil {
		t.Errorf("service failed to start")
	}

	time.Sleep(time.Second)
	if !ctx.IsActive("goTestTrue") {
		t.Errorf("service should be active")
	}
	if ctx.IsActive("goTestFalse") {
		t.Errorf("service should not be active")
	}
}

//func IsFailed(service string) bool
func TestIsFailed(t *testing.T) {
	if !IsSystemd() {
		t.Skipf("Not running systemd")
	}
	if IsFailed("init.scope") {
		t.Errorf("init.scope failed?!")
	}
	if IsFailed("g1bb3r15h.invalid") {
		t.Errorf("nonsense service 'g1bb3r15h.invalid' is failed?!")
	}

	//test against services we create
	ctx := UserContext()
	if ctx.IsFailed("goTestTrue") {
		t.Errorf("service should not be failed")
	}
	if !ctx.IsFailed("goTestFalse") {
		t.Errorf("service should be failed")
	}
}

//func Failed() (list []string)
func TestFailed(t *testing.T) {
	if !IsSystemd() {
		t.Skipf("Not running systemd")
	}
	t.Logf("manual verification required")
	t.Log(Failed())
}

var falseSvc, trueSvc string

func setup() {
	home := os.Getenv("HOME")
	svcdir := fp.Join(home, ".config", "systemd", "user")
	fi, err := os.Stat(svcdir)
	if err != nil {
		panic(err)
	}
	if !fi.IsDir() {
		panic("svcdir: not a dir")
	}

	trueSvc = fp.Join(svcdir, "goTestTrue.service")
	falseSvc = fp.Join(svcdir, "goTestFalse.service")

	trueExe, err := exec.LookPath("true")
	if err != nil {
		panic(err)
	}
	falseExe, err := exec.LookPath("false")
	if err != nil {
		panic(err)
	}

	unit := `[Unit]
Description=useless service (for testing)
After=network.target

[Service]
ExecStart=%s
RemainAfterExit=true
`
	err = ioutil.WriteFile(trueSvc, []byte(fmt.Sprintf(unit, trueExe)), 0644)
	if err != nil {
		panic(err)
	}
	err = ioutil.WriteFile(falseSvc, []byte(fmt.Sprintf(unit, falseExe)), 0644)
	if err != nil {
		panic(err)
	}
	ctx := UserContext()
	_ = ctx.sysctlCmdErr("reset-failed", fp.Base(trueSvc))
	_ = ctx.sysctlCmdErr("reset-failed", fp.Base(falseSvc))
}
func teardown() {
	ctx := UserContext()
	if err := ctx.Stop(fp.Base(trueSvc)); err != nil {
		panic(err)
	}
	if err := ctx.Stop(fp.Base(falseSvc)); err != nil {
		panic(err)
	}
	_ = ctx.sysctlCmdErr("reset-failed", fp.Base(trueSvc))
	_ = ctx.sysctlCmdErr("reset-failed", fp.Base(falseSvc))
	if err := os.Remove(trueSvc); err != nil {
		panic(err)
	}
	if err := os.Remove(falseSvc); err != nil {
		panic(err)
	}
}
