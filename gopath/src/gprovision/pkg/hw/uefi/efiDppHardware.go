// Copyright (C) 2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package uefi

import (
	"fmt"
)

type EfiDppHwSubType EfiDevPathProtoSubType

const (
	DppHTypePCI EfiDppHwSubType = iota + 1
	DppHTypePCCARD
	DppHTypeMMap
	DppHTypeVendor
	DppHTypeCtrl
	DppHTypeBMC
)

func (s EfiDppHwSubType) String() string {
	switch s {
	case DppHTypePCI:
		return "PCI"
	case DppHTypePCCARD:
		return "PCCARD"
	case DppHTypeMMap:
		return "MMap"
	case DppHTypeVendor:
		return "Vendor"
	case DppHTypeCtrl:
		return "Control"
	case DppHTypeBMC:
		return "BMC"
	default:
		return fmt.Sprintf("UNKNOWN-0x%x", uint8(s))
	}
}

//struct in EfiDevicePathProtocol for DppHTypePCI
type DppHwPci struct {
	Hdr              EfiDevicePathProtocolHdr
	Function, Device uint8
}

var _ EfiDevicePathProtocol = (*DppHwPci)(nil)

func ParseDppHwPci(h EfiDevicePathProtocolHdr, b []byte) (*DppHwPci, error) {
	if len(b) != 2 {
		return nil, EParse
	}
	return &DppHwPci{
		Hdr:      h,
		Function: b[0],
		Device:   b[1],
	}, nil
}
func (e *DppHwPci) Header() EfiDevicePathProtocolHdr { return e.Hdr }
func (e *DppHwPci) ProtoSubTypeStr() string {
	return EfiDppHwSubType(e.Hdr.ProtoSubType).String()
}
func (e *DppHwPci) String() string {
	return fmt.Sprintf("PCI(0x%x,0x%x)", e.Function, e.Device)
}

func (e *DppHwPci) Resolver() (EfiPathSegmentResolver, error) {
	return nil, EUnimpl
}
