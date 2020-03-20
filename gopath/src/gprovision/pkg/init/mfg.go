// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

// +build mfg

package init

import (
	"gprovision/pkg/hw/power"
	"gprovision/pkg/log"
	"gprovision/pkg/mfg"
	"os"
	"os/exec"

	"github.com/u-root/u-root/pkg/mount"
)

func stage2(uproc *os.Process) {
	if verbose {
		log.Logf("mode: mfg")
	}
	shell := os.Getenv("SHELL")
	if shell == "shell" {
		oob := os.Getenv("OOB")
		if oob == "y" {
			err := os.MkdirAll("/mnt/oob", 0755)
			if err != nil {
				log.Logf("creating oob dir: %s", err)
			}
			err = mount.Mount("OOB", "/mnt/oob", "9p", "", 0)
			if err != nil {
				log.Logf("mount oob: %s", err)
			}
		}
		sh := exec.Command("setsid", "cttyhack", "/bin/sh")
		sh.Stdin = os.NewFile(0, "stdin")
		sh.Stdout = os.NewFile(1, "stdout")
		sh.Stderr = os.NewFile(2, "stderr")
		err := sh.Run()
		if err != nil {
			log.Logf("run sh: %s", err)
		}
	} else {
		mfg.Main()
	}
	power.RebootSuccess()
}
