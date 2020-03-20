// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package appliance

import (
	"fmt"
	"gprovision/pkg/common/strs"
	futil "gprovision/pkg/fileutil"
	"gprovision/pkg/log"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
)

/*  locate* functions
 *
 *  look through available devices for one with correct size, connection type, etc
 *  return dev string, i.e. '/dev/sdc'
 */

func locateByLabel(media recoveryMediaS) (fsIdent, fsType, fsOpts string) {
	path := "/dev/disk/by-label/" + strs.RecVolName()
	log.Msgf("Waiting for recovery to appear...")
	if futil.WaitFor(path, 10*time.Second) {
		dev, err := filepath.EvalSymlinks(path)
		if err != nil {
			log.Logf("FindRecovery: error %s / dev %#v", err, dev)
			return
		}
		err = media.ValidateFn(dev)
		if err != nil {
			log.Logf("error finding recovery dev: found device has wrong attribute(s), err %s", err)
			return
		}
	}
	fsIdent = path
	fsType = media.FsType
	fsOpts = media.FsOpts()
	return
}

//9p2k virtfs, qemu/kvm
//validate9p is not called from locate func
func locate9PvirtRecov(media recoveryMediaS) (fsIdent, fsType, fsOpts string) {
	fsIdent = strs.RecVolName()
	fsType = media.FsType
	fsOpts = media.FsOpts()
	return
}

func (m recoveryMediaS) FsOpts() (fsOpts string) {
	fsOpts = m.FsOptsAdditional
	if !m.FsOptsOverride {
		fsOpts += "," + StandardMountOpts
	}
	if m.SSD {
		fsOpts += ",discard"
	}
	return
}

/*  validate* functions
 *
 *  check given device to ensure it meets requirements specific to a platform for one with correct size, connection type, etc
 *  return dev string, i.e. '/dev/sdc'
 */

//check if CFEX device (Portwell CAF-2000)
func validateCFEX(dev string) (err error) {
	//appears to be a SATA device
	link := "/sys/block/" + stripDev(dev)
	dest, _ := os.Readlink(link) //.../pcinnn:nn/nnn:nn.n/ataN/hostN/targetN:n:n/n:n/...
	ata := strings.Contains(dest, "/ata")
	//how reliable will checking the model prefix be?
	model, _ := ioutil.ReadFile(filepath.Join(dest, "device", "model"))
	CFEX := strings.HasPrefix(string(model), "CFX")
	//TODO other checks? size?
	if !ata || !CFEX {
		err = fmt.Errorf("validCFEX - no match: dev=%s,link=%s,dest=%s,model=%s,ata=%t,CFEX=%t",
			dev, link, dest, model, ata, CFEX)
	}
	return
}
func validateSATA(dev string) (err error) {
	link := "/sys/block/" + stripDev(dev)
	dest, _ := os.Readlink(link) //.../pcinnn:nn/nnn:nn.n/ataN/hostN/targetN:n:n/n:n/...
	ata := strings.Contains(dest, "/ata")

	if !ata {
		err = fmt.Errorf("validateSATA - no match: dev=%s,link=%s,dest=%s,ata=%t",
			dev, link, dest, ata)
	}
	return
}

//check if USB device
func validateUSB(dev string) (err error) {
	link := "/sys/block/" + stripDev(dev)
	dest, _ := os.Readlink(link)
	usb := strings.Contains(dest, "/usb")
	//dmidecode check internal port location??
	if !usb {
		err = fmt.Errorf("validUSB - no match: dev=%s,link=%s,dest=%s", dev, link, dest)
	}
	return
}

// always returns an error. this is ok since, when locating 9p recovery, the
// validate fn is only called from the locate fn and locate9p does not use this.
// still useful to have for testing erase functionality.
func validate9P(dev string) (err error) {
	return fmt.Errorf("9p never matches a device")
}

func stripDev(d string) string {
	return strings.TrimRight(strings.TrimPrefix(d, "/dev/"), "0123456789")
}
