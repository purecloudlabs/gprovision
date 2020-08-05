// Copyright (C) 2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package uefi

import (
	"bytes"
	"fmt"
)

type EfiDppACPISubType EfiDevPathProtoSubType

const (
	DppAcpiTypeDevPath EfiDppACPISubType = iota + 1
	DppAcpiTypeExpandedDevPath
	DppAcpiTypeADR
	DppAcpiTypeNVDIMM
)

func (e EfiDppACPISubType) String() string {
	switch e {
	case DppAcpiTypeDevPath:
		return "Device Path"
	case DppAcpiTypeExpandedDevPath:
		return "Expanded Device Path"
	case DppAcpiTypeADR:
		return "_ADR"
	case DppAcpiTypeNVDIMM:
		return "NVDIMM"
	default:
		return fmt.Sprintf("UNKNOWN-0x%x", uint8(e))
	}
}

type DppAcpiDevPath struct {
	Hdr      EfiDevicePathProtocolHdr
	HID, UID []byte //both length 4; not sure of endianness
}

var _ EfiDevicePathProtocol = (*DppMsgATAPI)(nil)

func ParseDppAcpiDevPath(h EfiDevicePathProtocolHdr, b []byte) (*DppAcpiDevPath, error) {
	if h.Length != 12 {
		return nil, EParse
	}
	return &DppAcpiDevPath{
		Hdr: h,
		HID: b[:4],
		UID: b[4:8],
	}, nil
}

func (e *DppAcpiDevPath) Header() EfiDevicePathProtocolHdr { return e.Hdr }
func (e *DppAcpiDevPath) ProtoSubTypeStr() string {
	return EfiDppACPISubType(e.Hdr.ProtoSubType).String()
}
func (e *DppAcpiDevPath) String() string                            { return fmt.Sprintf("ACPI(0x%x,0x%x)", e.HID, e.UID) }
func (e *DppAcpiDevPath) Resolver() (EfiPathSegmentResolver, error) { return nil, EUnimpl }

type DppAcpiExDevPath struct {
	Hdr                    EfiDevicePathProtocolHdr
	HID, UID, CID          []byte //all length 4; not sure of endianness
	HIDSTR, UIDSTR, CIDSTR string
}

var _ EfiDevicePathProtocol = (*DppMsgATAPI)(nil)

func ParseDppAcpiExDevPath(h EfiDevicePathProtocolHdr, b []byte) (*DppAcpiExDevPath, error) {
	if h.Length < 19 {
		return nil, EParse
	}
	ex := &DppAcpiExDevPath{
		Hdr: h,
		HID: b[:4],
		UID: b[4:8],
		CID: b[8:12],
	}
	b = b[12:]
	var err error
	ex.HIDSTR, err = readToNull(b)
	if err != nil {
		return nil, err
	}
	b = b[len(ex.HIDSTR)+1:]
	ex.UIDSTR, err = readToNull(b)
	if err != nil {
		return nil, err
	}
	b = b[len(ex.UIDSTR)+1:]
	ex.CIDSTR, err = readToNull(b)
	if err != nil {
		return nil, err
	}
	return ex, nil
}

func (e *DppAcpiExDevPath) Header() EfiDevicePathProtocolHdr { return e.Hdr }
func (e *DppAcpiExDevPath) ProtoSubTypeStr() string {
	return EfiDppACPISubType(e.Hdr.ProtoSubType).String()
}
func (e *DppAcpiExDevPath) String() string {
	return fmt.Sprintf("ACPI_EX(0x%x,0x%x,0x%x,%s,%s,%s)", e.HID, e.UID, e.CID, e.HIDSTR, e.UIDSTR, e.CIDSTR)
}

func (e *DppAcpiExDevPath) Resolver() (EfiPathSegmentResolver, error) {
	return nil, EUnimpl
}

func readToNull(b []byte) (string, error) {
	i := bytes.IndexRune(b, 0)
	if i < 0 {
		return "", EParse
	}
	return string(b[:i]), nil
}
