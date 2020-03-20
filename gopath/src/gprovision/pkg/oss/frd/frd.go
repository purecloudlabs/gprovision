// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

// Minimal, OSS impl of a mechanism storing data for use by factory restore. If
// you accept files from user-inserted media, you are responsible for ensuring
// they cannot cause undesired behavior.
package frd

import (
	"encoding/json"
	"errors"
	"gprovision/pkg/common"
	"gprovision/pkg/common/fr"
	"gprovision/pkg/common/rlog"
	"gprovision/pkg/hw/beep"
	"gprovision/pkg/hw/cfa"
	"gprovision/pkg/hw/ipmi/uid"
	"gprovision/pkg/hw/power"
	"gprovision/pkg/log"
	"gprovision/pkg/net"
	"io/ioutil"
	"os"
	fp "path/filepath"
	"sync"
	"time"
)

const frName = "fr.data"

var (
	frOnce     sync.Once
	ENotLoaded = errors.New("FRData not loaded")
)

// Called from package that needs to use frdata, generally from init() func.
func UseImpl() {
	frOnce.Do(func() { fr.SetImpl(&frd{}) })
}

type frd struct {
	u      common.Unit
	loaded bool
	Data   Frjson //data that may be persisted
}

type Frjson struct {
	Preserve       bool
	IgnoreNetCfg   bool
	XLog, BootArgs string
}

// Volatile storage of unit info for use by other methods. Never persisted.
func (d *frd) SetUnit(u common.Unit) { d.u = u }

// Load FRData from user-inserted media or from recovery volume; return error
// if unable to decode.
func (d *frd) ReadRecoveryOr(userFiles []string) error {
	if len(userFiles) != 0 {
		//only read user files if you sanitize them and/or sign and verify them
		log.Log("not reading untrusted user files")
	}
	return d.Read()
}

// Store FRData.
func (d *frd) Persist() error { return persist(d.u.Rec.Path(), d.Data) }

func persist(dir string, data Frjson) error {
	fname := fp.Join(dir, frName)
	dat, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(fname, dat, 0644)
}

// Load persisted FRData.
func (d *frd) Read() error {
	fname := fp.Join(d.u.Rec.Path(), frName)
	data, err := ioutil.ReadFile(fname)
	if err != nil {
		return err
	}
	err = json.Unmarshal(data, &d.Data)
	if err != nil {
		return err
	}
	d.loaded = true
	return nil
}

// Delete persisted FRData if its persist flag is unset.
func (d *frd) Delete() error {
	if !d.loaded {
		return ENotLoaded
	}
	if !d.Data.Preserve {
		return os.Remove(fp.Join(d.u.Rec.Path(), frName))
	}
	log.Logf("not deleting fr data")
	return nil
}

// Handle is called by factory restore. Handles the options present in
// loaded config - for example,
// - configure remote logging
// - delete network config that was saved
// - etc
func (d *frd) Handle() error {
	log.Logf("frd handle: %#v", *d)
	if d.Data.XLog != "" {
		//enable networking...
		success := net.EnableNetworkingSkipDIAG(d.u.Platform.DiagPorts(), d.u.Platform.MACPrefixes())
		if !success && d.u.Platform.IsPrototype() {
			log.Logf("Are network ports set up correctly? Pausing for 1 minute...")
			done := make(chan struct{})
			go beep.BeepUntil(done, time.Second*4)
			go uid.BlinkUntil(done, 4)
			_ = cfa.DefaultLcd.BlinkMsg("Network port issue, pausing...", cfa.Fade, time.Second*2, time.Minute)
			close(done)
			success = net.EnableNetworkingAny()
		}
		if !success {
			log.Logf("Network error, cannot log")
			done := make(chan struct{})
			go beep.BeepUntil(done, time.Second*4)
			go uid.BlinkUntil(done, 4)
			_ = cfa.DefaultLcd.BlinkMsg("Network error, cannot log", cfa.Fade, time.Second*2, 48*time.Hour)
			close(done)
			power.Reboot(false)
		}
		//...and logging
		return rlog.Setup(d.Data.XLog, d.u.Platform.SerNum())
	}
	return nil
}

// If true, should delete any saved network config
func (d *frd) IgnoreNetworkConfig() bool {
	return d.Data.IgnoreNetCfg
}

// If true, saved network config will be deleted. Not part of the interface
// since it is not used within this codebase.
func (d *frd) SetIgnoreNetCfg(ignore bool) {
	d.Data.IgnoreNetCfg = ignore
}

// Store extra boot args for the grub menu. Useful in development.
func (d *frd) SetBootArgs(bootArgs string) {
	d.Data.BootArgs = bootArgs
}

// Retrieve extra boot args, if any.
func (d *frd) AdditionalBootArgs() string {
	return d.Data.BootArgs
}

// Set url used for external logging. Used for first (in-house) factory restore.
func (d *frd) SetXLog(url string) {
	d.Data.XLog = url
}

// If true, Delete method will have no effect in this session or any other.
// Useful in development.
func (d *frd) SetPreserve(noDelete bool) {
	d.Data.Preserve = noDelete
}
