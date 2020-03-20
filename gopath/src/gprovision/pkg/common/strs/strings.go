// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

//Abstraction for strings that implementors will likely wish to change.
package strs

import (
	"gprovision/pkg/log"
	fp "path/filepath"
	"strings"
)

//Abstraction for strings that implementors will likely wish to change.
type Stringer interface {
	//Dir within the image where config data is stored. Likely under /etc.
	ConfDir() string
	//Prefix used for env vars.
	EnvPrefix() string
	//Name of the recovery volume.
	RecVolName() string
	//Name of the primary volume.
	PriVolName() string
	//Prefix used in creating hostname. Note hostname charset restrictions.
	HostPrefix() string
	//Name of file whose absence triggers factory restore.
	FlagFile() string
	//name of boot kernel
	BootKernel() string
	//name of provisioning/mfg kernel
	MfgKernel() string
	// Returns prefix for image name. 3 sections, 2 dots + 1 trailing. Otherwise
	// sort function will fail.
	ImgPrefix() string
	//prefix to require on MAC addresses
	MacOUI() string
	//like MacOUI, but as bytes
	MacOUIBytes() []byte
	//Filename prefix to match a file which can be used as an emergency image.
	EmergPfx() string
	//dir on recovery volume to use for logs
	RecoveryLogDir() string
}

var stringImpl Stringer

//Override defaults.
func SetStringer(b Stringer) {
	if stringImpl != nil {
		log.Log("strs: overriding non-nil impl")
	}
	stringImpl = b
}

//Dir within the image where config data is stored. Likely under /etc.
func ConfDir() string {
	if stringImpl != nil {
		return stringImpl.ConfDir()
	}
	return "/etc/provisioning"
}

//Prefix used for env vars.
func EnvPrefix() string {
	if stringImpl != nil {
		return stringImpl.EnvPrefix()
	}
	return "PROVISION_"
}

//Name of the recovery volume.
func RecVolName() string {
	if stringImpl != nil {
		return stringImpl.RecVolName()
	}
	return "RECOVERY"
}

//Name of the primary volume.
func PriVolName() string {
	if stringImpl != nil {
		return stringImpl.PriVolName()
	}
	return "PRIMARY"
}

//Prefix used in creating hostname. Note hostname charset restrictions.
func HostPrefix() string {
	if stringImpl != nil {
		return stringImpl.HostPrefix()
	}
	return "gp-"
}

//Name of file whose absence triggers factory restore.
func FlagFile() string {
	if stringImpl != nil {
		return stringImpl.FlagFile()
	}
	return "boot.normal"
}

//name of boot kernel
func BootKernel() string {
	if stringImpl != nil {
		return stringImpl.BootKernel()
	}
	return "norm_boot"
}

//name of provisioning/mfg kernel
func MfgKernel() string {
	if stringImpl != nil {
		return stringImpl.MfgKernel()
	}
	return "provision.pxe"
}

// Returns prefix for image name. 3 sections, 2 dots + 1 trailing. Otherwise
// sort function will fail.
func ImgPrefix() string {
	if stringImpl != nil {
		return stringImpl.ImgPrefix()
	}
	return "WIDGET.LNX.SHINY."
}

//prefix to require on MAC addresses
func MacOUI() string {
	if stringImpl != nil {
		return stringImpl.MacOUI()
	}
	return "56:78:90"
}

//like MacOUI, but as bytes
func MacOUIBytes() []byte {
	if stringImpl != nil {
		return stringImpl.MacOUIBytes()
	}
	return []byte{0x56, 0x78, 0x90}
}

func EmergPfx() string {
	if stringImpl != nil {
		return stringImpl.EmergPfx()
	}
	return "EmergencyFile_"
}

func RecoveryLogDir() string {
	if stringImpl != nil {
		return stringImpl.RecoveryLogDir()
	}
	return "log"
}

func MfgLogPfx() string {
	logPfx := MfgKernel()
	return strings.TrimSuffix(logPfx, fp.Ext(logPfx))
}

func FRLogPfx() string { return "recov" }
