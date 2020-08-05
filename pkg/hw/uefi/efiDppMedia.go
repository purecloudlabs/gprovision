// Copyright (C) 2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package uefi

import (
	"encoding/binary"
	"fmt"
	"os"
	fp "path/filepath"
	"strings"

	"github.com/purecloudlabs/gprovision/pkg/guid"
	"github.com/purecloudlabs/gprovision/pkg/hw/block"
	"github.com/purecloudlabs/gprovision/pkg/log"
)

type EfiDppMediaSubType EfiDevPathProtoSubType

const (
	//DppTypeMedia, pg 319 +
	DppMTypeHdd      EfiDppMediaSubType = iota + 1 //0x01
	DppMTypeCd                                     //0x02
	DppMTypeVendor                                 //0x03
	DppMTypeFilePath                               //0x04 //p321
	DppMTypeMedia                                  //0x05 //media protocol i.e. filesystem format??
	DppMTypePIWGFF                                 //0x06
	DppMTypePIWGFV                                 //0x07
	DppMTypeRelOff                                 //0x08
	DppMTypeRAM                                    //0x09
)

func (e EfiDppMediaSubType) String() string {
	switch e {
	case DppMTypeHdd:
		return "HDD"
	case DppMTypeCd:
		return "CD"
	case DppMTypeVendor:
		return "Vendor"
	case DppMTypeFilePath:
		return "FilePath"
	case DppMTypeMedia:
		return "Media"
	case DppMTypePIWGFF:
		return "PIWG Firmware File"
	case DppMTypePIWGFV:
		return "PIWG Firmware Volume"
	case DppMTypeRelOff:
		return "Relative Offset"
	case DppMTypeRAM:
		return "RAMDisk"
	default:
		return fmt.Sprintf("UNKNOWN-0x%x", uint8(e))
	}
}

//struct in EfiDevicePathProtocol for DppMTypeHdd
type DppMediaHDD struct {
	Hdr EfiDevicePathProtocolHdr

	PartNum   uint32         //index into partition table for MBR or GPT; 0 indicates entire disk
	PartStart uint64         //starting LBA. only used for MBR?
	PartSize  uint64         //size in LB's. only used for MBR?
	PartSig   guid.MixedGuid //format determined by SigType below. unused bytes must be 0x0.
	PartFmt   uint8          //0x01 for MBR, 0x02 for GPT
	SigType   uint8          //0x00 - none; 0x01 - 32bit MBR sig (@ 0x1b8); 0x02 - GUID
}

var _ EfiDevicePathProtocol = (*DppMediaHDD)(nil)

func ParseDppMediaHdd(h EfiDevicePathProtocolHdr, b []byte) (*DppMediaHDD, error) {
	if len(b) < 38 {
		return nil, EParse
	}
	hdd := &DppMediaHDD{
		Hdr:       h,
		PartNum:   binary.LittleEndian.Uint32(b[:4]),
		PartStart: binary.LittleEndian.Uint64(b[4:12]),
		PartSize:  binary.LittleEndian.Uint64(b[12:20]),
		//PartSig:   b[20:36], //cannot assign slice to array
		PartFmt: b[36],
		SigType: b[37],
	}
	copy(hdd.PartSig[:], b[20:36])
	return hdd, nil
}

func (e *DppMediaHDD) Header() EfiDevicePathProtocolHdr { return e.Hdr }
func (e *DppMediaHDD) ProtoSubTypeStr() string {
	return EfiDppMediaSubType(e.Hdr.ProtoSubType).String()
}
func (e *DppMediaHDD) String() string {
	//             (part#,pttype,guid,begin,length)
	return fmt.Sprintf("HD(%d,%s,%s,0x%x,0x%x)", e.PartNum, e.pttype(), e.sig(), e.PartStart, e.PartSize)
}

func (e *DppMediaHDD) Resolver() (EfiPathSegmentResolver, error) {
	var blkfilter block.BlkIncludeFn
	if e.SigType == 2 {
		guid := e.PartSig.ToStdEnc().String()
		blkfilter = func(bi block.BlkInfo) bool { return bi.PartUUID == guid }
	} else if e.SigType == 1 {
		//apparently, for MBR blkid reports PARTUUID as mbrId-partNum
		id := fmt.Sprintf("%x-%02d", e.PartSig[:4], e.PartNum)
		blkfilter = func(bi block.BlkInfo) bool { return bi.PartUUID == id }
	} else {
		//no sig, would need to compare partition #/start/len
		//difficult... fall back to alt methods in this case?
		log.Logf("Sig Type 0: cannot identify")
		return nil, ENotFound
	}
	blocks := block.GetFilesystems(blkfilter /*block.DFiltOnlyUsb*/, nil)
	if len(blocks) != 1 {
		log.Logf("blocks: %#v", blocks)
		return nil, ENotFound
	}
	return &HddResolver{BlkInfo: blocks[0]}, nil
}

//return the partition table type as a string
func (e *DppMediaHDD) pttype() string {
	switch e.PartFmt {
	case 1:
		return "MBR"
	case 2:
		return "GPT"
	default:
		return "UNKNOWN"
	}
}

//return the signature as a string
func (e *DppMediaHDD) sig() string {
	switch e.SigType {
	case 1: //32-bit MBR sig
		return fmt.Sprintf("%x", binary.LittleEndian.Uint32(e.PartSig[:4]))
	case 2: //GUID
		return e.PartSig.ToStdEnc().String()
	default:
		return "(NO SIG)"
	}
}

//struct in EfiDevicePathProtocol for DppMTypeFilePath.
//if multiple are included in a load option, concatenate them.
type DppMediaFilePath struct {
	Hdr EfiDevicePathProtocolHdr

	PathNameDecoded string //stored as utf16
}

var _ EfiDevicePathProtocol = (*DppMediaFilePath)(nil)

func ParseDppMediaFilePath(h EfiDevicePathProtocolHdr, b []byte) (*DppMediaFilePath, error) {
	if len(b) < int(h.Length)-4 {
		return nil, EParse
	}
	path, err := DecodeUTF16(b[:h.Length-4])
	if err != nil {
		return nil, err
	}
	//remove null termination byte, replace windows slashes
	path = strings.TrimSuffix(path, "\000")
	path = strings.Replace(path, "\\", string(os.PathSeparator), -1)
	fp := &DppMediaFilePath{
		Hdr:             h,
		PathNameDecoded: path,
	}
	return fp, nil
}

func (e *DppMediaFilePath) Header() EfiDevicePathProtocolHdr { return e.Hdr }
func (e *DppMediaFilePath) ProtoSubTypeStr() string {
	return EfiDppMediaSubType(e.Hdr.ProtoSubType).String()
}
func (e *DppMediaFilePath) String() string {
	return fmt.Sprintf("File(%s)", e.PathNameDecoded)
}

func (e *DppMediaFilePath) Resolver() (EfiPathSegmentResolver, error) {
	fp.Clean(e.PathNameDecoded)
	pr := PathResolver(e.PathNameDecoded)
	return &pr, nil
}

//struct in EfiDevicePathProtocol for DppMTypePIWGFV
type DppMediaPIWGFV struct {
	Hdr EfiDevicePathProtocolHdr
	Fv  []byte
}

var _ EfiDevicePathProtocol = (*DppMediaPIWGFV)(nil)

func ParseDppMediaPIWGFV(h EfiDevicePathProtocolHdr, b []byte) (*DppMediaPIWGFV, error) {
	if h.Length != 20 {
		return nil, EParse
	}
	fv := &DppMediaPIWGFV{
		Hdr: h,
		Fv:  b,
	}
	return fv, nil
}
func (e *DppMediaPIWGFV) Header() EfiDevicePathProtocolHdr { return e.Hdr }
func (e *DppMediaPIWGFV) ProtoSubTypeStr() string {
	return EfiDppMediaSubType(e.Hdr.ProtoSubType).String()
}
func (e *DppMediaPIWGFV) String() string {
	var g guid.MixedGuid
	copy(g[:], e.Fv)
	return fmt.Sprintf("Fv(%s)", g.ToStdEnc().String())
}

func (e *DppMediaPIWGFV) Resolver() (EfiPathSegmentResolver, error) {
	return nil, EUnimpl
}

//struct in EfiDevicePathProtocol for DppMTypePIWGFF
type DppMediaPIWGFF struct {
	Hdr EfiDevicePathProtocolHdr
	Ff  []byte
}

var _ EfiDevicePathProtocol = (*DppMediaPIWGFF)(nil)

func ParseDppMediaPIWGFF(h EfiDevicePathProtocolHdr, b []byte) (*DppMediaPIWGFF, error) {
	if h.Length != 20 {
		return nil, EParse
	}
	fv := &DppMediaPIWGFF{
		Hdr: h,
		Ff:  b,
	}
	return fv, nil
}
func (e *DppMediaPIWGFF) Header() EfiDevicePathProtocolHdr { return e.Hdr }
func (e *DppMediaPIWGFF) ProtoSubTypeStr() string {
	return EfiDppMediaSubType(e.Hdr.ProtoSubType).String()
}
func (e *DppMediaPIWGFF) String() string {
	var g guid.MixedGuid
	copy(g[:], e.Ff)
	return fmt.Sprintf("FvFile(%s)", g.ToStdEnc().String())
}

func (e *DppMediaPIWGFF) Resolver() (EfiPathSegmentResolver, error) {
	return nil, EUnimpl
}
