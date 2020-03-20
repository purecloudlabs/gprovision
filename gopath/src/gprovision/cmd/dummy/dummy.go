// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

// A helper/dummy multi-call binary used in integ testing. Dummy because the
// functionality implied by the binaries' name(s) is not present. Used in
// integration testing.
//
// Fake systemd
//
// This cmd is compiled and written to /usr/lib/systemd/systemd in a fake .upd
// file, printing out messages to show that it was executed.
//
// Fake chpasswd
//
// This cmd is compiled and written to /bin/chpasswd in a fake .upd file. Note
// that while it is in the fake update tarball, it runs from within factory
// restore.
//
// Used in integration testing.
package main

import (
	"fmt"
	"gprovision/pkg/appliance"
	"gprovision/pkg/common/rlog"
	"gprovision/pkg/log"
	"gprovision/pkg/log/flags"
	"gprovision/testing/fakeupd"
	"os"
	fp "path/filepath"
	"time"

	"github.com/u-root/u-root/pkg/mount"
	"golang.org/x/sys/unix"
)

var logger string //can be set by linker, see fakeupd.go

func main() {
	log.AddConsoleLog(flags.NA)
	switch fp.Base(os.Args[0]) {
	case "systemd":
		fakeSysd()
	case "chpasswd":
		fakeChpass()
	default:
		panic(fmt.Sprintf("dummy multi-call binary: unknown name %s", os.Args[0]))
	}
}

func fakeSysd() {
	if len(logger) > 0 {
		v := appliance.Read()
		log.SetPrefix("fakeSysd")
		err := rlog.Setup(logger, v.SerNum())
		if err != nil {
			log.Fatalf("log setup: %s", err)
		}
	}
	//can't merely write to console - must write to serial port.
	//before we can do that, remount root without the readonly flag
	err := mount.Mount("", "/", "", "", unix.MS_REMOUNT)
	if err != nil {
		log.Logln(err)
		time.Sleep(time.Minute)
	}
	//create character device node with correct major/minor numbers 4, 64
	err = unix.Mknod("/dev/ttyS0", 0660|unix.S_IFCHR, int(unix.Mkdev(4, 64)))
	if err != nil {
		log.Logln(err)
	}
	ttys, err := os.OpenFile("/dev/ttyS0", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Logln(err)
	} else {
		os.Stdout = ttys
		os.Stderr = ttys
	}
	//write messages
	log.Log(fakeupd.InitRunning)
	time.Sleep(time.Second)
	log.Log(fakeupd.Bye)
	//shut down
	err = unix.Reboot(unix.LINUX_REBOOT_CMD_POWER_OFF)
	if err != nil {
		log.Logf("%s", err)
	}
}

func fakeChpass() {
	if len(logger) > 0 {
		v := appliance.Read()
		log.SetPrefix("fakeChpasswd")
		err := rlog.Setup(logger, v.SerNum())
		if err != nil {
			log.Fatalf("log setup: %s", err)
		}
	}
	log.Log("fake chpasswd run")
}
