// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

// +build !release

package init

import (
	"gprovision/pkg/common/strs"
	"gprovision/pkg/log"
	"os"
	"time"

	"golang.org/x/sys/unix"
)

const release = false

func handleEnvVars() {
	if os.Getpid() != 1 {
		os.Setenv(strs.VerboseEnv(), "1")
		os.Setenv(strs.NoRebootEnv(), "1")
	}
	if len(os.Args) > 1 {
		log.Logf("args: %#v", os.Args)
		time.Sleep(10 * time.Second)
	}
	commonEnvVars()
}

//if the integ test env var is set to a recognized value, do something and do not return
func testOpts() {
	switch os.Getenv(strs.IntegEnv()) {
	case "ck":
		log.Logf("INTEG_TEST: ck")
		if fsKey != nil {
			//load fs encryption key
			fsKey.LoadKey()
		}
	default:
		return
	}
	log.Logf("INTEG_TEST: done")
	err := unix.Reboot(unix.LINUX_REBOOT_CMD_POWER_OFF)
	if err != nil {
		log.Logf("%s", err)
	}
	os.Exit(1)
}
