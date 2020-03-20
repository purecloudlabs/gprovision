// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

// Package erase handles data erase of data on units, for use when a
// customer's data is sufficiently sensitive and they need to ship a unit
// without data. It uses the drive's ATA SECURE ERASE command when available,
// falling back to writing pattterns if that command cannot be used.
//
// Before erase, canary values are written at predetermined points on disk.
// After erase, it reads the locations those values were written, verifying
// they no longer exist. In the unlikely event that the canary values still
// exist, this indicates that at least some disk areas were not successfully
// erased. In this case, the boot menu file is modified to cause the unit to
// only display a warning message when it boots. The warning message explains
// that data erase failed, sensitive data may remain, and that they must
// contact support. The unit will not do anything more than display this message
// until it has been RMA'd, QA'd, and re-imaged. This is by design, to ensure
// the problem is resolved; after all, the customer's data was sensitive enough
// to warrant a multi-hour erase process.
package erase

import (
	"bytes"
	"fmt"
	"gprovision/pkg/appliance"
	"gprovision/pkg/common"
	"gprovision/pkg/common/strs"
	"gprovision/pkg/erase/raid"
	"gprovision/pkg/hw/cfa"
	"gprovision/pkg/hw/udev"
	hk "gprovision/pkg/init/housekeeping"
	"gprovision/pkg/log"
	logflags "gprovision/pkg/log/flags"
	"gprovision/pkg/log/lcd"
	"gprovision/pkg/recovery/disk"
	"io"
	"os"
	"os/exec"
	fp "path/filepath"
	"sync"
	"time"
)

const (
	nrOverwritePatterns = 3
)

var s cfa.Spinner

var Platform *appliance.Variant

func Main() {
	log.AddConsoleLog(logflags.EndUser)
	var err error
	_, err = cfa.FindWithRetry()
	if err != nil {
		log.Logf("finding lcd: %s", err)
	}
	if cfa.DefaultLcd != nil {
		_ = lcd.AddLcdLog(logflags.EndUser)
	}

	log.Msg("identifying platform...")
	Platform, err = appliance.IdentifyWithFallback(disk.PlatIdentFromRecovery)
	if err != nil {
		log.Logf("identifying platform: %s", err)
	}
	log.Msg("identified")

	if !udev.IsRunning() {
		_, _ = udev.Start()
		log.Log("started udev")
	}
	recov := disk.FindRecovery(Platform)

	hk.AddPrebootDefaults(disk.UnmountAll)

	if recov.Valid() {
		recov.Mount()
		log.Msg("recovery volume found")
	} else {
		log.Msg("can't find recovery")
	}
	Erase(recov)
}

func Erase(recov common.FS) {

	//if kernel is booted with key=val, this is added to the environment
	//check for a var signalling that we've already had an unrecoverable error
	//if we have, display that message for a long time, then reboot
	str := os.Getenv(strs.EraseEnv())
	if str == unrecoverableErrVal {
		unrecoverableFailure(recov, false)
	}

	logRoot := ""
	if recov.IsMounted() {
		log.Msg("Recovery media at " + recov.Path())
		logRoot = fp.Join(recov.Path(), strs.RecoveryLogDir())
	}
	log.SetPrefix("data_erase")
	if log.InStack(log.FileLogIdent) {
		log.Msg("already logging to file, not creating new file log")
	} else {
		_, _ = log.AddFileLog(logRoot)
	}
	log.FlushMemLog()

	log.Msg("Data erase: locating drives...")

	//find all disks
	devs, err := raid.FindDevices(Platform)
	if err != nil {
		log.Logf("raid.FindDevices: err %s", err)
	}
	//sort disks into potential arrays, based on size/metadata type
	arrs := raid.FindArrays(devs)
	for i, a := range arrs {
		log.Logf("array %d: type %s", i, a.Type())
	}
	if len(arrs) != 1 {
		log.Logf("wrong number of arrays. Want 1, got %d\n%#v", len(arrs), arrs)
		unrecoverableFailure(recov, true)
	}
	a := arrs[0]
	if a.Len() != Platform.DataDisks() {
		log.Logf("wrong number of drives. Want %d, got %d", Platform.DataDisks(), a.Len())
		unrecoverableFailure(recov, true)
	}
	err = a.Backup()
	if err != nil {
		log.Msg("error creating backup")
		log.Logf("Array.Backup: err %s", err)
		//exit? retry?
	}

	var wg sync.WaitGroup

	s.Msg = "Erasing..."
	s.Lcd = cfa.DefaultLcd
	_ = s.Display()

	eraseCh := make(chan time.Duration, 2)
	defer close(eraseCh)
	go eraseStatus(eraseCh, 4*time.Hour, &s)

	for _, a := range arrs {
		for _, d := range a.Devices() {
			wg.Add(1)
			go eraseDisk(d, &wg, eraseCh, recov)
		}
	}
	wg.Wait()
	close(eraseCh)

	log.Msg("Writing RAID config...")
	for i, a := range arrs {
		e := a.Restore()
		if e != nil {
			log.Logf("failed to re-write array data for %d: %s", i, e)
		}
	}
	success(recov)
}

/*hdparm -I output
 *
...
Security:
       Master password revision code = 65534
               supported
       not     enabled
       not     locked
       not     frozen
       not     expired: security count
               supported: enhanced erase
       274min for SECURITY ERASE UNIT. 274min for ENHANCED SECURITY ERASE UNIT.
*
*/
func eraseDisk(d *raid.Device, wg *sync.WaitGroup, eraseCh chan<- time.Duration, recov common.FS) {
	defer wg.Done()
	prepare(d, recov)
	d.Close() //not sure what happens if we have an open fd when running hdparm
	if tryhdp(d, eraseCh) != nil {
		overwrite(d, eraseCh)
	}
	verify(d, recov)
}

// overwrite all data on disk with various patterns
// use if hdparm fails
func overwrite(d *raid.Device, eraseCh chan<- time.Duration) {
	log.Logf("%s: pattern overwrite", d.Dev())
	buf := make([]byte, 4096*1024)
	eraseCh <- 12 * time.Hour
	data := bytes.NewReader(buf)
	dev, err := d.Open()
	if err != nil {
		return
	}
	//loop over erase patterns
	var p int
	var count int
	for p = 0; p <= nrOverwritePatterns; p++ {
		fillPattern(buf, p)
		//loop until entire device has been overwritten
		if _, err := dev.Seek(0, 0); err != nil {
			log.Logf("dev seek: %s", err)
		}
		for {
			count++
			if _, err := data.Seek(0, 0); err != nil {
				log.Logf("data seek: %s", err)
			}
			_, err := io.Copy(dev, data)
			if err != nil {
				log.Logf("%s: stopping with %s", d.Dev(), err)
				break
			}
		}
	}
}

//use hdparm + ATA SECURE ERASE to erase the disk
func tryhdp(d *raid.Device, eraseCh chan<- time.Duration) error {
	log.Logf("%s: trying ATA SECURE ERASE command", d.Dev())
	drvInfo := exec.Command("hdparm", "-I", d.Dev())
	info, err := tryExec(drvInfo, 10, 2*time.Second)
	if err != nil {
		log.Logf("error %s executing %#v\noutput:\n%s", err, drvInfo.Args, string(info))
		return err
	}
	froz := getLine(info, "frozen")
	supported := bytes.Contains(froz, []byte("not"))
	if !supported {
		log.Logf("%s: ATA SECURE ERASE command isn't supported: %s", d.Dev(), info)
		return fmt.Errorf("unsupported")
	}

	eraseOpt := getLine(info, "enhanced erase")
	enhancedErase := (len(eraseOpt) > 20) && !bytes.Contains(eraseOpt, []byte("not"))

	//find time data in 'info', communicate it to lcd update thread
	eraseCh <- getSEtime(info, enhancedErase)

	setPass := exec.Command("hdparm", "--security-set-pass", "fsadfsfd", d.Dev())
	out, err := tryExec(setPass, 10, 2*time.Second)
	if err != nil {
		log.Logf("error %s executing %#v\noutput:\n%s", err, setPass.Args, string(out))
		return err
	}
	var eraseArg string
	if enhancedErase {
		eraseArg = "--security-erase-enhanced"
	} else {
		eraseArg = "--security-erase"
	}
	erase := exec.Command("hdparm", eraseArg, "fsadfsfd", d.Dev())
	out, err = tryExec(erase, 10, 2*time.Second)
	if err != nil {
		log.Logf("error '%s' executing %#v\noutput:\n%s", err, erase.Args, string(out))
	}
	return err
}

//hdparm seems to fail intermittently, so try to run it several times
func tryExec(ex *exec.Cmd, max int, wait time.Duration) (combinedOutput []byte, err error) {
	count := 1
	for count <= max {
		combinedOutput, err = ex.CombinedOutput()
		if err == nil {
			if count > 1 {
				log.Logln("tryExec: ", ex.Args, " suceeded after ", count, " tries")
			}
			break
		}
		count++
		time.Sleep(wait)

		/* exec.Cmd struct retains some state data; running
		 * a second time will result in errors... so start over
		 */
		args := ex.Args
		ex = new(exec.Cmd)
		ex.Args = args
		ex.Path = args[0]
	}
	return
}
