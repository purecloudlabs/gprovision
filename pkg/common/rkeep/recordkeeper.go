// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

//Some sort of external mechanism recording details about units imaged.
//Could be part of your remote logger, or separate.
package rkeep

import (
	"github.com/purecloudlabs/gprovision/pkg/common"
	"github.com/purecloudlabs/gprovision/pkg/log"
)

//Some sort of external mechanism recording details about units imaged.
//Could be part of your remote logger, or separate.
type RecordKeeper interface {
	//called once serial number (etc) is known
	SetUnit(u common.Unit)

	//Store MACs
	StoreMACs([]string)
	//Store IPMI MACs
	StoreIPMIMACs([]string)

	//current process finished; reboot pending
	ReportFinished(string)

	//current process failed
	ReportFailure(string)

	//report codename of current device
	ReportCodename(string)

	// Gets time from an external source that has drift correction, unlike the
	// RTC in an unpowered server. Format 2006-01-02 15:04:05
	GetTime() string

	StoreDocument(name string, doctype PrintedDocType, doc []byte)

	// FIXME needs rethought - why GetRecordKeeper when we can instead assign
	// to RKeeper when provData is loaded?
	//Called in ProvisioningData.GetRecordKeeper()
	//FIXME what is needed?
	//SetData(pd ProvisioningData)
}

type PrintedDocType string

const (
	PrintedDocUnknown PrintedDocType = "unknown"
	PrintedDocQAV     PrintedDocType = "QA Verification"
)

var rkeeper RecordKeeper

//Sets the underlying RecordKeeper impl for this package
func SetImpl(r RecordKeeper) {
	if rkeeper != nil {
		log.Log("RecordKeeper: overwriting non-nil impl")
	}
	rkeeper = r
}

//Return true if an impl is set
func HaveRKeeper() bool { return rkeeper != nil }

//called once serial number (etc) is known
func SetUnit(u common.Unit) {
	if rkeeper != nil {
		rkeeper.SetUnit(u)
		return
	}
	log.Log("RecordKeeper impl unset")
}

//Store MACs
func StoreMACs(m []string) {
	if rkeeper != nil {
		rkeeper.StoreMACs(m)
		return
	}
	log.Log("RecordKeeper impl unset")
}

//Store IPMI MACs
func StoreIPMIMACs(m []string) {
	if rkeeper != nil {
		rkeeper.StoreIPMIMACs(m)
		return
	}
	log.Log("RecordKeeper impl unset")
}

//current process finished; reboot pending
func ReportFinished(f string) {
	if rkeeper != nil {
		rkeeper.ReportFinished(f)
		return
	}
	log.Log("RecordKeeper impl unset")
}

//current process failed
func ReportFailure(f string) {
	if rkeeper != nil {
		rkeeper.ReportFailure(f)
		return
	}
	log.Log("RecordKeeper impl unset")
}

//report codename of current device
func ReportCodename(c string) {
	if rkeeper != nil {
		rkeeper.ReportCodename(c)
		return
	}
	log.Log("RecordKeeper impl unset")
}

// Gets time from an external source that has drift correction, unlike the
// RTC in an unpowered server. Format 2006-01-02 15:04:05
func GetTime() string {
	if rkeeper != nil {
		return rkeeper.GetTime()
	}
	log.Log("RecordKeeper impl unset")
	return ""
}

func StoreDocument(name string, doctype PrintedDocType, doc []byte) {
	if rkeeper != nil {
		rkeeper.StoreDocument(name, doctype, doc)
		return
	}
	log.Log("RecordKeeper impl unset")
}
