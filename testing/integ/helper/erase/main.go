// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

// +build !release

package main

import (
	"os"

	"github.com/purecloudlabs/gprovision/pkg/appliance"
	"github.com/purecloudlabs/gprovision/pkg/hw/power"
	"github.com/purecloudlabs/gprovision/pkg/hw/udev"
	ginit "github.com/purecloudlabs/gprovision/pkg/init"
	"github.com/purecloudlabs/gprovision/pkg/log"
	"github.com/purecloudlabs/gprovision/pkg/recovery/disk"
)

//set eraseHelper=verify to verify pattern
func main() {
	os.Setenv("PATH", "/sbin:/bin:/usr/bin:/usr/sbin")
	log.AddConsoleLog(0)
	log.FlushMemLog()

	ginit.CreateDirs()
	ginit.EarlyMounts()

	_, err := udev.Start()
	if err != nil {
		log.Logf("starting udev: %s", err)
	}

	plat := appliance.Identify()
	if plat == nil {
		log.Logf("plat is nil")
		power.FailReboot()
	}
	log.Log("primary/main volume...")
	disks := disk.FindTargets(plat)

	if len(disks) != 1 {
		log.Fatalf("need exactly 1 disk got %d", len(disks))
	}
	mode := os.Getenv("eraseHelper")
	if mode == "verify" {
		verify(plat, disks)
	} else {
		prepare(plat, disks)
	}
	power.Off()
}
