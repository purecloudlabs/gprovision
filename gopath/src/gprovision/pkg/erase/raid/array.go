// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

// Package raid handles reading, storing, and re-writing raid metadata as
// necessary during the erase process. It handles iMSM and SNIA DDF formats.
package raid

import (
	"gprovision/pkg/log"
	"strings"
)

type Array struct {
	arrayType raidType
	devices   Devices
}

/* look through Device list, sort disks into potential arrays (based on size/metadata type)
 * does NOT actually decode the metadata... so don't use for anything other than erasure!
 */
func FindArrays(devices Devices) (arrays Arrays) {
	var a *Array
	for i, d := range devices {
		if d.InArray() || d.arrayType == unset {
			continue
		}
		a = NewArray(d)

		for _, e := range devices[i:] {
			if e.InArray() {
				continue
			}
			if e.arrayType == a.arrayType && sizeMatch(d, e) {
				if err := a.Add(e); err != nil {
					log.Logf("array add: %s", err)
				}
			}
		}
		arrays = append(arrays, a)
	}
	return
}

func NewArray(d *Device) (a *Array) {
	a = new(Array)
	a.devices = append(a.devices, d)
	a.arrayType = d.arrayType
	d.array = a
	return
}

func (a *Array) Devices() Devices {
	return a.devices
}

func (a *Array) Add(d *Device) error {
	if d.arrayType != a.arrayType {
		return ETypeMismatch
	}
	a.devices = append(a.devices, d)
	d.array = a
	return nil
}

func (a *Array) Backup() (err error) {
	for _, d := range a.devices {
		err = d.Backup()
		if err != nil {
			return
		}
	}
	return
}

//executes restore on all drives in array, returning error from last error-ing drive or nil if no errors
func (a *Array) Restore() (err error) {
	for _, d := range a.devices {
		e := d.Restore()
		if e != nil {
			log.Logf("array.Restore, dev %s: err %s", d.dev, e)
			err = e
		}
	}
	return
}

func (a *Array) Type() string {
	return a.arrayType.String()
}

func (a *Array) Len() int {
	return len(a.devices)
}
func (a *Array) String() string { return "{\n" + a.devices.String() + "\n}" }

type Arrays []*Array

func (as Arrays) String() string {
	var str []string
	for _, a := range as {
		str = append(str, a.String())
	}
	return strings.Join(str, "\n")
}
