// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package stash

import (
	"github.com/purecloudlabs/gprovision/pkg/common"
	"github.com/purecloudlabs/gprovision/pkg/log"
	steps "github.com/purecloudlabs/gprovision/pkg/mfg/configStep"
)

// Stasher securely stores secrets and allows agonizing abominable alliteration.
// It stores things such as keys, certificates, passwords on the unit. Where
// these things come from is implementation-defined: they could come from a
// local CSPRNG, an external server, etc.
//
// Stasher must be able to write all secrets locally, but the only type for
// which reading is supported in this interface is passwords. Reading passwords
// is necessary so they can be set, and when making out-of-band changes (i.e.
// IPMI, BIOS).
type Stasher interface {
	//set serial number, recovery volume, etc
	SetUnit(unit common.Unit)
	//called immediately after mfg data is parsed
	SetData(common.StashData)

	// Determine unit credentials, store securely. Sets IPMI/BIOS pw
	// using an OOB tool.
	HandleCredentials(cfgSteps steps.ConfigSteps)

	//Stores other secrets.
	Mfg()

	//Returns OS Password.
	ReadOSPass() (string, error)
	//Returns BIOS Password.
	ReadBiosPass() (string, error)
	//Returns IPMI Password.
	ReadIPMIPass() (string, error)

	// Asks user to input shell password. Compares to stored pw. Reboots if no
	// match - ONLY returns if password matches.
	RequestShellPassword()
}

var stasherImpl Stasher

//sets the underlying Stasher implementation for this package
func SetImpl(s Stasher) {
	if stasherImpl != nil {
		log.Log("Stasher: overwriting non-nil impl")
	}
	stasherImpl = s
}

//set serial number, recovery volume, etc
func SetUnit(unit common.Unit) {
	if stasherImpl != nil {
		stasherImpl.SetUnit(unit)
	} else {
		log.Log("Stasher: impl unset")
	}
}

//called immediately after mfg data is parsed
func SetData(d common.StashData) {
	if stasherImpl != nil {
		stasherImpl.SetData(d)
	} else {
		log.Log("Stasher: impl unset")
	}
}

// Determine unit credentials, store securely. Sets IPMI/BIOS pw
// using an OOB tool.
func HandleCredentials(cfgSteps steps.ConfigSteps) {
	if stasherImpl != nil {
		stasherImpl.HandleCredentials(cfgSteps)
	} else {
		log.Log("Stasher: impl unset")
	}
}

//Stores other secrets.
func Mfg() {
	if stasherImpl != nil {
		stasherImpl.Mfg()
	} else {
		log.Log("Stasher: impl unset")
	}
}

//Returns OS Password.
func ReadOSPass() (string, error) {
	if stasherImpl != nil {
		return stasherImpl.ReadOSPass()
	}
	log.Log("Stasher: impl unset")
	return "", nil
}

//Returns BIOS Password.
func ReadBiosPass() (string, error) {
	if stasherImpl != nil {
		return stasherImpl.ReadBiosPass()
	}
	log.Log("Stasher: impl unset")
	return "", nil
}

//Returns IPMI Password.
func ReadIPMIPass() (string, error) {
	if stasherImpl != nil {
		return stasherImpl.ReadIPMIPass()
	}
	log.Log("Stasher: impl unset")
	return "", nil
}

// Asks user to input shell password. Compares to stored pw. Reboots if no
// match - ONLY returns if password matches.
func RequestShellPassword() {
	if stasherImpl != nil {
		stasherImpl.RequestShellPassword()
	} else {
		log.Log("Stasher: impl unset")
	}
}
