// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

//Package udev allows starting udev and finding udevd if running.
package udev

import (
	"fmt"
	"gprovision/pkg/log"
	"os"
	"os/exec"
	"strings"
)

var ExecErr = fmt.Errorf("execution error")

//Start udevd, return its os.Process so it can be killed later.
func Start() (*os.Process, error) {
	udevd := exec.Command("udevd", "--resolve-names=never")
	log.Logf("Running %v...", udevd.Args)
	err := udevd.Start()
	if err != nil {
		log.Logf("error %s running %v", err, udevd.Args)
		return nil, err
	}

	_, success := log.Cmd(exec.Command("udevadm", "trigger", "--type=subsystems", "--action=add"))
	if !success {
		return nil, ExecErr
	}
	_, success = log.Cmd(exec.Command("udevadm", "trigger", "--type=devices", "--action=add"))
	if !success {
		return nil, ExecErr
	}
	_, success = log.Cmd(exec.Command("udevadm", "settle", "--timeout=20"))
	if !success {
		return nil, ExecErr
	}

	return udevd.Process, nil
}

//return true if udevd appears to be running
//udev could already be running, particularly if we're in emergency mode.
//naive impl, would see any process with name/args ending in 'udevd' as being the udev daemon
func IsRunning() bool {
	out, err := exec.Command("/bin/busybox", "ps", "aux").Output()
	if err != nil {
		log.Logf("running ps aux: %s", err)
		return false
	}
	lines := strings.Split(string(out), "\n")
	for _, l := range lines {
		if strings.Contains(l, "udevd") {
			return true
		}
	}
	return false
}
