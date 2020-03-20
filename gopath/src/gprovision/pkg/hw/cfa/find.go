// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package cfa

import (
	"fmt"
	"gprovision/pkg/log"
	"io/ioutil"
	"os"
	fp "path/filepath"
	"strings"
	"time"
)

var EMissing = fmt.Errorf("No LCD found")

//packages can use this rather than calling Find() and maintaining a local *Lcd.
var DefaultLcd *Lcd

//Preferred method of getting an Lcd{}. Finds lcd serial port, connects to it.
//If one has already been found, return that. Stores mutable state so it can be
//written back later.
func Find() (*Lcd, error) {
	var err error
	if DefaultLcd == nil {
		devs := FindDevs()
		if len(devs) == 0 {
			err = EMissing
		} else {
			DefaultLcd, err = ConnectTo(devs[0])
		}
	}
	return DefaultLcd, err
}

var gaveUpSearch bool

//like Find() but tries multiple times.
func FindWithRetry() (*Lcd, error) {
	var err error
	if DefaultLcd == nil && !gaveUpSearch {
		for i := 5; i > 0; i-- {
			_, err = Find()
			var olderr error
			if err != nil {
				if err != olderr {
					log.Logf("Finding lcd: %s", err)
					olderr = err
				}
				log.Logf("Retrying (%d)...", i)
				time.Sleep(time.Second)
			} else {
				break
			}
		}
		if err != nil {
			log.Logf("giving up on lcd init")
			gaveUpSearch = true
		}
	}
	return DefaultLcd, err
}

//Called as factory restore and mfg exit
func Uninit(_ bool) {
	if DefaultLcd != nil {
		DefaultLcd.Close()
		DefaultLcd = nil //prevent uninit code from running multiple times
	}
}
func (l *Lcd) Uninit(_ bool) {
	if l == nil {
		return
	}
	l.Close()
}

//Connect to an lcd given its port name, such as '/dev/ttyACM0'.
//Stores mutable state so it can be written back later.
func ConnectTo(dev string) (*Lcd, error) {
	sd, err := SerialSetup(dev)
	if err != nil {
		return nil, err
	}
	return connectTo(sd)
}
func connectTo(sd *SerialDev) (l *Lcd, err error) {
	l = &Lcd{dev: sd}
	info, err := l.Revision()
	if err == nil {
		if strings.HasPrefix(strings.ToUpper(info), "CFA635") {
			l.model = Cfa635
		}
		//anything else is treated as a 631
	}
	l.dims.Col = 19
	l.dims.Row = 1
	if l.model == Cfa635 {
		l.dims.Row = 3
	}
	err = l.captureState()
	if err != nil {
		log.Logf("failed to capture lcd state: %s", err)
	}
	if l.model == Cfa635 {
		l.dev.XlateTable = xlate635
	} else {
		l.dev.XlateTable = xlate631
	}
	return
}

//used in FindDevs to print fewer messages
var failedToLocate bool

//look for usb serial devices that use the cdc or ftdi driver and appear to be Crystalfontz LCDs.
func FindDevs() (devs []string) {
	devs = findNewDevs()
	devs = append(devs, findOldDevs()...)
	for _, d := range devs {
		log.Logf("Found LCD at %s", d)
	}
	if len(devs) > 0 {
		return
	}
	if !failedToLocate {
		log.Logf("Failed to locate LCD. Guessing...")
		failedToLocate = true
	}
	for _, d := range []string{
		"/dev/ttyACM0", //new
		"/dev/ttyUSB0", //old
	} {
		_, err := os.Stat(d)
		if err == nil {
			log.Logf("%s exists and could be an LCD...", d)
			return []string{d}
		}
	}
	return
}

//old crystalfontz devices using FTDI driver, /dev/ttyUSB*
func findOldDevs() (devs []string) {
	entries, _ := fp.Glob("/sys/bus/usb-serial/drivers/ftdi_sio/tty*")
	for _, e := range entries {
		p, err := fp.EvalSymlinks(e)
		if err != nil {
			log.Logf("%s: symlink err %s\n", e, err)
			continue
		}
		m := fp.Join(p, "../../manufacturer")
		mfg, err := ioutil.ReadFile(m)
		if err != nil {
			log.Logf("%s: mfg file err %s\n", e, err)
			continue
		}
		lower := strings.ToLower(string(mfg))
		if strings.TrimSpace(lower) == "crystalfontz" {
			dev := "/dev/" + fp.Base(e)
			devs = append(devs, dev)
		} else {
			log.Logf("skipping %s with mfg %s\n", e, string(mfg))
		}
	}
	return
}

//find new crystalfontz devices using CDC ACM driver, /dev/ttyACM*
func findNewDevs() (devs []string) {
	//it seems there will be 2 dirs for each device. look for one with a tty subdir.
	entries, _ := fp.Glob("/sys/bus/usb/drivers/cdc_acm/*/tty/tty*")
	for _, e := range entries {
		node := fp.Base(e)
		p, err := fp.EvalSymlinks(fp.Join(e, "device"))
		if err != nil {
			log.Logf("%s: symlink err %s\n", e, err)
			continue
		}
		prod, err := ioutil.ReadFile(fp.Join(p, "../idProduct"))
		if err != nil {
			log.Logf("%s: read product err %s\n", e, err)
			continue
		}
		vend, err := ioutil.ReadFile(fp.Join(p, "../idVendor"))
		if err != nil {
			log.Logf("%s: read vendor err %s\n", e, err)
			continue
		}
		if strings.TrimSpace(string(prod)) == "000b" &&
			strings.TrimSpace(string(vend)) == "223b" {
			devs = append(devs, "/dev/"+node)
		} else {
			log.Logf("skipping %s:%s\n", vend, prod)
		}
	}
	return
}
