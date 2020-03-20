// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

// Provides feedback on the LCD that the boot is progressing.
package progress

import (
	"gprovision/pkg/common/strs"
	"gprovision/pkg/hw/cfa"
	"gprovision/pkg/hw/kmsg"
	initconsts "gprovision/pkg/init/consts"
	"gprovision/pkg/log"
	"gprovision/pkg/log/lcd"
	"os"
	"os/exec"
	fp "path/filepath"
	"sync"
	"time"
)

const (
	progressProc = "lcdBootProgress"

	normBootMsg  = "OS starting..."
	firstBootMsg = "Preparing fresh OS image..."
	endMsg       = "Services starting..."
)

var fbaDone = fp.Join(strs.ConfDir(), "configure-done")

// Call to start displaying progress as systemd starts up. Does nothing (and
// returns immediately) if argv[0] matches progressProc; otherwise never
// returns. Displays animation until reciept of SIGUSR1 or until 5 minutes
// lapse, whichever comes first.
func MaybeStart() {
	km := kmsg.NewKmsgPrio(kmsg.FacLocal0, kmsg.SevNotice, progressProc)
	if os.Args[0] != progressProc {
		if pid := os.Getpid(); pid != 1 {
			km.Logf("warning, pid = %d", pid)
		}
		return
	}
	log.AddConsoleLog(0)
	km.Printf("running...")

	lcd, err := cfa.Find()
	if err != nil {
		km.Printf("finding lcd: %s", err)
		os.Exit(1)
	}
	if lcd == nil {
		km.Printf("no lcd")
		os.Exit(1)
	}

	sig := setupSignal()

	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)

	msg := normBootMsg
	if !normBoot() {
		msg = firstBootMsg
	}

	go func() {
		lcd.SpinnerUntil(msg, time.Second/2, done)
		wg.Done()
	}()

	waitForSignal(km, sig, time.Minute*5)

	close(done)
	wg.Wait()
	_, _ = lcd.Msg(endMsg)
	lcd.Close()
	os.Exit(0)
}

// Call before SwitchRoot to spawn secondary process that will update LCD as
// systemd starts. Secondary process will run Start().
func Fork() {
	km := kmsg.NewKmsgPrio(kmsg.FacSys, kmsg.SevNotice, "init")
	if cfa.DefaultLcd == nil {
		km.Logf("no lcd, not forking progress process")
		return
	}
	log.RemoveLogger(lcd.LcdLogIdent)
	cfa.DefaultLcd.Close()
	cfa.DefaultLcd = nil

	cmd := exec.Command("/init")
	cmd.Args[0] = progressProc
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	km.Logf("spawn %s...", progressProc)

	err := cmd.Start()
	if err != nil {
		km.Logf("failed to fork %s: %s", progressProc, err)
	}
}

// differentiate between normal boot and first boot
func normBoot() bool {
	path := fp.Join(initconsts.NewRoot, fbaDone)
	_, err := os.Stat(path)
	return err == nil
}
