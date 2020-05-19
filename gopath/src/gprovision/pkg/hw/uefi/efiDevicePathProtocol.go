// Copyright (C) 2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package uefi

import (
	"errors"
	"fmt"
	"gprovision/pkg/log"
	"strings"
)

var (
	Verbose bool

	EParse    = errors.New("parse error")
	ENotFound = errors.New("Described device not found")
	EUnimpl   = errors.New("Not implemented")
)

func ParseFilePathList(in []byte) (EfiDevicePathProtocolList, error) {
	reachedEnd := false
	b := in
	var list EfiDevicePathProtocolList
loop:
	for len(b) >= 4 {
		h := EfiDevicePathProtocolHdr{
			ProtoType:    EfiDevPathProtoType(b[0]),
			ProtoSubType: EfiDevPathProtoSubType(b[1]),
			Length:       bytesToU16(b[2:4]),
		}
		if h.Length < 4 {
			log.Logf("invalid struct - len %d remain %d: 0x%x", h.Length, len(b), b)
			return nil, EParse
		}
		if len(b) < int(h.Length) {
			log.Logf("undersize %s: %d < %d %x\nin %q", h.ProtoType, len(b)+4, h.Length, b, in)
			return nil, EParse
		}
		data := b[4:h.Length]
		b = b[h.Length:]
		var p EfiDevicePathProtocol
		var err error
		switch h.ProtoType {
		case DppTypeHw:
			st := EfiDppHwSubType(h.ProtoSubType)
			if Verbose {
				log.Logf("hw subtype %s", st)
			}
			switch st {
			case DppHTypePCI:
				p, err = ParseDppHwPci(h, data)
			// case DppHTypePCCARD:
			// case DppHTypeMMap:
			// case DppHTypeVendor:
			// case DppHTypeCtrl:
			// case DppHTypeBMC:
			default:
				log.Logf("unhandled hw subtype %s: %q", st, data)
			}
			if err != nil {
				log.Logf("%s %s: %s", h.ProtoType, st, err)
				return nil, err
			}
		case DppTypeACPI:
			st := EfiDppACPISubType(h.ProtoSubType)
			if Verbose {
				log.Logf("hw subtype %s", st)
			}
			switch st {
			case DppAcpiTypeDevPath:
				p, err = ParseDppAcpiDevPath(h, data)
			case DppAcpiTypeExpandedDevPath:
				p, err = ParseDppAcpiExDevPath(h, data)
			default:
				log.Logf("unhandled acpi subtype %s: %q", st, data)
			}
			if err != nil {
				log.Logf("%s %s: %s", h.ProtoType, st, err)
				return nil, err
			}
		case DppTypeMessaging:
			st := EfiDppMsgSubType(h.ProtoSubType)
			if Verbose {
				log.Logf("msg subtype %s", st)
			}
			switch st {
			case DppMsgTypeATAPI:
				p, err = ParseDppMsgATAPI(h, data)
			case DppMsgTypeMAC:
				p, err = ParseDppMsgMAC(h, data)
			default:
				log.Logf("unhandled msg subtype %s: %q", st, data)
			}
			if err != nil {
				log.Logf("%s %s: %s", h.ProtoType, st, err)
				return nil, err
			}

		case DppTypeMedia:
			st := EfiDppMediaSubType(h.ProtoSubType)
			if Verbose {
				log.Logf("media subtype %s", st)
			}
			switch st {
			case DppMTypeHdd:
				p, err = ParseDppMediaHdd(h, data)
			case DppMTypeFilePath:
				p, err = ParseDppMediaFilePath(h, data)
			case DppMTypePIWGFF:
				p, err = ParseDppMediaPIWGFF(h, data)
			case DppMTypePIWGFV:
				p, err = ParseDppMediaPIWGFV(h, data)
			default:
				log.Logf("unhandled media subtype %s: %q", st, data)
			}
			if err != nil {
				log.Logf("%s %s: %s", h.ProtoType, st, err)
				return nil, err
			}
		case DppTypeEnd:
			//should be last item on list
			reachedEnd = true
			st := EfiDppEndSubType(h.ProtoSubType)
			if st != DppETypeEndEntire {
				log.Logf("unexpected end subtype %s", st)
			}
			break loop
		default:
			log.Logf("unhandled type %s", h.ProtoType)
		}
		if p == nil {
			p = &EfiDevPathRaw{
				Hdr: h,
				Raw: data,
			}
		}
		list = append(list, p)
	}
	if !reachedEnd {
		log.Logf("FilePathList incorrectly terminated")
		return nil, EParse
	}
	if len(b) != 0 {
		log.Logf("remaining bytes %x", b)
		return nil, EParse
	}
	return list, nil
}

//An EfiDevicePathProtocol identifies a device path.
type EfiDevicePathProtocol interface {
	Header() EfiDevicePathProtocolHdr

	//subtype as human readable
	ProtoSubTypeStr() string

	//path as human readable
	String() string

	//returns an EfiPathSegmentResolver; in the case of filesystems, this locates and mounts the device.
	Resolver() (EfiPathSegmentResolver, error)
}

type EfiDevicePathProtocolList []EfiDevicePathProtocol

func (list EfiDevicePathProtocolList) String() string {
	var res string
	for n, dpp := range list {
		if dpp == nil {
			log.Fatalf("nil dpp %d %#v", n, list)
			continue
		}
		res += dpp.String() + "/"
	}
	return strings.Trim(res, "/")
}

// EfiDevicePathProtocolHdr - all variants start with the following three fields:
//    typedef struct _EFI_DEVICE_PATH_PROTOCOL {
//        UINT8 Type;
//        UINT8 SubType;
//        UINT8 Length[2];
//    } EFI_DEVICE_PATH_PROTOCOL;
// It seems that the only relevant Type (for booting) is media.
//
// https://uefi.org/sites/default/files/resources/UEFI_Spec_2_8_A_Feb14.pdf
// pg 286 +
type EfiDevicePathProtocolHdr struct {
	ProtoType    EfiDevPathProtoType
	ProtoSubType EfiDevPathProtoSubType
	Length       uint16
}

type EfiDevPathProtoType uint8

const (
	DppTypeHw        EfiDevPathProtoType = iota + 1 //0x01, pg 288
	DppTypeACPI                                     //0x02, pg 290
	DppTypeMessaging                                //0x03, pg 293
	DppTypeMedia                                    //0x04, pg 319
	DppTypeBBS                                      //0x05, pg 287
	DppTypeEnd       EfiDevPathProtoType = 0x7f
)

func (e EfiDevPathProtoType) String() string {
	switch e {
	case DppTypeHw:
		return "HW"
	case DppTypeACPI:
		return "ACPI"
	case DppTypeMessaging:
		return "Messaging"
	case DppTypeMedia:
		return "Media"
	case DppTypeBBS:
		return "BBS"
	case DppTypeEnd:
		return "End"
	default:
		return fmt.Sprintf("UNKNOWN-0x%x", uint8(e))
	}
}

//we only define media and end subtypes; others exist in spec
type EfiDevPathProtoSubType uint8

type EfiDppEndSubType EfiDevPathProtoSubType

const (
	//DppTypeEnd, pg 287-288
	DppETypeEndStartNew EfiDppEndSubType = 0x01 //only for DppTypeHw?
	DppETypeEndEntire   EfiDppEndSubType = 0xff
)

func (e EfiDppEndSubType) String() string {
	switch e {
	case DppETypeEndEntire:
		return "End"
	case DppETypeEndStartNew:
		return "End one, start another"
	default:
		return fmt.Sprintf("UNKNOWN-0x%x", uint8(e))
	}
}

//marks end of EfiDevicePathProtocol
type EfiDevPathEnd struct {
	Hdr EfiDevicePathProtocolHdr
}

var _ EfiDevicePathProtocol = (*EfiDevPathEnd)(nil)

func (e *EfiDevPathEnd) Header() EfiDevicePathProtocolHdr { return e.Hdr }
func (e *EfiDevPathEnd) ProtoSubTypeStr() string {
	return EfiDppEndSubType(e.Hdr.ProtoSubType).String()
}
func (e *EfiDevPathEnd) String() string { return "" }
func (e *EfiDevPathEnd) Resolver() (EfiPathSegmentResolver, error) {
	return nil, nil
}

type EfiDevPathRaw struct {
	Hdr EfiDevicePathProtocolHdr
	Raw []byte
}

func (e *EfiDevPathRaw) Header() EfiDevicePathProtocolHdr { return e.Hdr }
func (e *EfiDevPathRaw) ProtoSubTypeStr() string {
	return EfiDppEndSubType(e.Hdr.ProtoSubType).String()
}
func (e *EfiDevPathRaw) String() string {
	return fmt.Sprintf("RAW(%s,0x%x,%d,0x%x)", e.Hdr.ProtoType, e.Hdr.ProtoSubType, e.Hdr.Length, e.Raw)
}

func (e *EfiDevPathRaw) Resolver() (EfiPathSegmentResolver, error) {
	return nil, EParse
}

/* https://uefi.org/sites/default/files/resources/UEFI_Spec_2_8_A_Feb14.pdf
Boot0007* UEFI OS       HD(1,GPT,81635ccd-1b4f-4d3f-b7b7-f78a5b029f35,0x40,0xf000)/File(\EFI\BOOT\BOOTX64.EFI)..BO

00000000  01 00 00 00 5e 00 55 00  45 00 46 00 49 00 20 00  |....^.U.E.F.I. .|
00000010  4f 00 53 00 00 00[04 01  2a 00 01 00 00 00 40 00  |O.S.....*.....@.|
00000020  00 00 00 00 00 00 00 f0  00 00 00 00 00 00 cd 5c  |...............\|
00000030  63 81 4f 1b 3f 4d b7 b7  f7 8a 5b 02 9f 35 02 02  |c.O.?M....[..5..|
00000040  04 04 30 00 5c 00 45 00  46 00 49 00 5c 00 42 00  |..0.\.E.F.I.\.B.|
00000050  4f 00 4f 00 54 00 5c 00  42 00 4f 00 4f 00 54 00  |O.O.T.\.B.O.O.T.|
00000060  58 00 36 00 34 00 2e 00  45 00 46 00 49 00 00 00  |X.6.4...E.F.I...|
00000070  7f ff 04 00]00 00 42 4f                           |......BO|
                     ^     ^     ][ = end, beginning of dpp list
dpp's alone
00000000  04 01 2a 00 01 00 00 00  40 00 00 00 00 00 00 00  |..*.....@.......|
00000010  00 f0 00 00 00 00 00 00  cd 5c 63 81 4f 1b 3f 4d  |.........\c.O.?M|
00000020  b7 b7 f7 8a 5b 02 9f 35  02 02*04 04 30 00 5c 00  |....[..5....0.\.|
00000030  45 00 46 00 49 00 5c 00  42 00 4f 00 4f 00 54 00  |E.F.I.\.B.O.O.T.|
00000040  5c 00 42 00 4f 00 4f 00  54 00 58 00 36 00 34 00  |\.B.O.O.T.X.6.4.|
00000050  2e 00 45 00 46 00 49 00  00 00*7f ff 04 00        |..E.F.I.......|
                                        ^ *=new dpp (x2)
type       = 0x04 (media)
subtype    = 0x01 (hdd)
struct len = 42 bytes always
part num   = 0x01
part start = 0x40
part size  = 0xf000
part sig   = 0xCD5C63814F1B3F4DB7B7F78A5B029F35
part fmt   = 0x02 (GPT)
sig type   = 0x02 (GUID)
=====
type       = 0x04 (media)
subtype    = 0x04 (file path)
struct len = 0x0030 + 4
path       = \EFI\BOOT\BOOTX64.EFI
====
type       = 0x7f (end)
subtype    = 0xff (end of entire path)
struct len = 4
*/
