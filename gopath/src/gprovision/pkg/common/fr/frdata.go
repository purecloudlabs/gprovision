// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

//Interface for a mechanism storing data for use by factory restore
package fr

import (
	"gprovision/pkg/common"
	"gprovision/pkg/log"
)

//Interface for a mechanism storing data for use by factory restore
type FRData interface {
	//Volatile storage of unit info for use by other methods. Never persisted.
	SetUnit(u common.Unit)

	//Load FRData from user-inserted media or from recovery volume; return error if unable to decode
	ReadRecoveryOr(userFiles []string) error

	//Store FRData. Does not use user-inserted media.
	Persist() error

	//Load persisted FRData.
	Read() error
	//Delete persisted FRData if its persist flag is unset.
	Delete() error

	// Handle is called by factory restore. Handles the options present in
	// loaded config - for example,
	// - configure remote logging
	// - delete network config that was saved
	// - etc
	Handle() error

	// If true, should delete any saved network config
	IgnoreNetworkConfig() bool

	//Store extra boot args for the grub menu. Useful in development.
	SetBootArgs(bootArgs string)
	//Load extra boot args, if any.
	AdditionalBootArgs() string

	//Set url used for external logging. Used for first (in-house) factory restore.
	SetXLog(url string)

	// If true, Delete method will have no effect in this session or any other.
	// Useful in development.
	SetPreserve(noDelete bool)
}

var fRDataImpl FRData

func SetImpl(f FRData) {
	if fRDataImpl != nil {
		log.Log("FRData: overwriting non-nil impl")
	}
	fRDataImpl = f
}

func HaveImpl() bool { return fRDataImpl != nil }

//Volatile storage of unit info for use by other methods. Never persisted.
func SetUnit(u common.Unit) {
	if fRDataImpl != nil {
		fRDataImpl.SetUnit(u)
	} else {
		log.Log("FRData impl unset")
	}
}

//Load FRData from user-inserted media or from recovery volume; return error if unable to decode
func ReadRecoveryOr(userFiles []string) error {
	if fRDataImpl != nil {
		return fRDataImpl.ReadRecoveryOr(userFiles)
	}
	log.Log("FRData impl unset")
	return common.ENil
}

//Store FRData. Does not use user-inserted media.
func Persist() error {
	if fRDataImpl != nil {
		return fRDataImpl.Persist()
	}
	log.Log("FRData impl unset")
	return common.ENil
}

//Load persisted FRData.
func Read() error {
	if fRDataImpl != nil {
		return fRDataImpl.Read()
	}
	log.Log("FRData impl unset")
	return common.ENil
}

//Delete persisted FRData if its persist flag is unset.
func Delete() error {
	if fRDataImpl != nil {
		return fRDataImpl.Delete()
	}
	log.Log("FRData impl unset")
	return common.ENil
}

// Handle is called by factory restore. Handles the options present in loaded
// config - for example,
// - configure remote logging
// - delete network config that was saved
// - etc
func Handle() error {
	if fRDataImpl != nil {
		return fRDataImpl.Handle()
	}
	log.Log("FRData impl unset")
	return common.ENil
}

// If true, should delete any saved network config
func IgnoreNetworkConfig() bool {
	if fRDataImpl != nil {
		return fRDataImpl.IgnoreNetworkConfig()
	}
	log.Log("FRData impl unset")
	return false
}

// Store extra boot args for the grub menu. Useful in development.
func SetBootArgs(bootArgs string) {
	if fRDataImpl != nil {
		fRDataImpl.SetBootArgs(bootArgs)
	} else {
		log.Log("FRData impl unset")
	}
}

// Load extra boot args, if any.
func AdditionalBootArgs() string {
	if fRDataImpl != nil {
		return fRDataImpl.AdditionalBootArgs()
	}
	log.Log("FRData impl unset")
	return ""
}

//Set url used for external logging. Used for first (in-house) factory restore.
func SetXLog(url string) {
	if fRDataImpl != nil {
		fRDataImpl.SetXLog(url)
	} else {
		log.Log("FRData impl unset")
	}
}

// If true, Delete method will have no effect in this session or any other.
// Useful in development.
func SetPreserve(noDelete bool) {
	if fRDataImpl != nil {
		fRDataImpl.SetPreserve(noDelete)
	} else {
		log.Log("FRData impl unset")
	}
}
