// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package init

import (
	"bytes"
	"io/ioutil"
	"os"
	fp "path/filepath"
	"time"

	"github.com/purecloudlabs/gprovision/pkg/appliance"
	"github.com/purecloudlabs/gprovision/pkg/common/rlog"
	"github.com/purecloudlabs/gprovision/pkg/common/strs"
	"github.com/purecloudlabs/gprovision/pkg/log"
	"github.com/purecloudlabs/gprovision/pkg/net"
	"github.com/purecloudlabs/gprovision/pkg/recovery/disk"

	"github.com/u-root/u-root/pkg/mount"
)

//do more with env vars and logging - stuff that can't be done until after udev/early mounts
func handleEnvVarsPt2() {
	var success bool
	if os.Getenv(strs.IntegEnv()) != "" {
		success = outputTo9p()
	}
	toConsole := log.InStack(log.ConsoleLogIdent)
	if !success && toConsole {
		outputToSerial()
	}
	// xlog: external logging (via logserver) of early boot. This logging shouldn't
	// leak any secrets, but we probably still don't want customers seeing it all.
	if xl := os.Getenv(strs.LogEnv()); xl != "" {
		log.Logf("ext logging requested. setup...")
		plat, err := appliance.IdentifyWithFallback(disk.PlatIdentFromRecovery)
		if err != nil {
			log.Logf("identifying platform: %s", err)
		}
		if release && (plat == nil || plat.FamilyName() != "qemu") {
			/*
				On release builds, only allow logging on test vm's (family qemu).
				Note that normal customer vm's, even if on qemu, would not be
				this family unless the customer very carefully set dmi values.

				Since the integ test uses release binaries, can't simply limit
				to debug builds.
			*/
			log.Logf("logging request denied")
			return
		}
		var success bool
		var serNum string
		if plat != nil {
			success = net.EnableNetworkingSkipDIAG(plat.DiagPorts(), plat.MACPrefixes())
			serNum = plat.SerNum()
		} else {
			log.Msg("unrecognized platform")
			time.Sleep(10 * time.Second)
			success = net.EnableNetworkingAny()
			serNum = "UNKNOWN"
		}
		if !success {
			log.Msg("no network connectivity")
			time.Sleep(10 * time.Second)
		} else {
			if err = rlog.Setup(xl, serNum); err != nil {
				log.Logf("setting up remote logging: %s", err)
			}
		}
	}
}

func outputToSerial() {
	cl, err := ioutil.ReadFile("/proc/cmdline")
	if err != nil {
		log.Logf("reading /proc/cmdline: %s", err)
	} else {
		if bytes.Contains(cl, []byte("console=ttyS0")) {
			ttys, err := os.OpenFile("/dev/ttyS0", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
			if err != nil {
				log.Logf("opening ttyS0: %s", err)
			} else {
				os.Stdout = ttys
				os.Stderr = ttys
			}
		}
	}
}

func outputTo9p() bool {
	err := os.MkdirAll("/testdata", 0755)
	if err != nil {
		log.Logf("Couldn't create /testdata: %v", err)
		return false
	}
	err = mount.Mount("tmpdir", "/testdata", "9p", "", 0)
	if err != nil {
		log.Logf("Couldn't mount /testdata: %s", err)
		return false
	}
	outDir := "/testdata/log"
	err = os.Mkdir(outDir, 0777)
	if err != nil {
		log.Logf("cannot create log dir: %s", err)
		return false
	}
	soPath := fp.Join(outDir, "stdout")
	sePath := fp.Join(outDir, "stderr")
	var so, se *os.File
	so, err = os.Create(soPath)
	if err == nil {
		se, err = os.Create(sePath)
	}
	if err != nil {
		log.Logf("9p error: %s", err)
		return false
	}

	os.Stdout = so
	os.Stderr = se
	_, _ = log.AddFileLog(outDir)
	log.Logln("writing to", outDir)
	lim, _ := ioutil.ReadFile("/proc/self/limits")
	log.Logf("limits:\n%s", string(lim))
	return true
}
