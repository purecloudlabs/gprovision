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
)

type EfiDppMsgSubType EfiDevPathProtoSubType

const (
	DppMsgTypeATAPI      EfiDppMsgSubType = iota + 1
	DppMsgTypeSCSI                        //2
	DppMsgTypeFibreCh                     //3
	DppMsgTypeFirewire                    //4
	DppMsgTypeUSB                         //5
	DppMsgTypeIIO                         //6
	_                                     //7
	_                                     //8
	DppMsgTypeInfiniband                  //9
	DppMsgTypeVendor                      //10 //uart flow control, sas are subtypes
	DppMsgTypeMAC                         //11
	DppMsgTypeIP4                         //12
	DppMsgTypeIP6                         //13
	DppMsgTypeUART                        //14
	DppMsgTypeUSBClass                    //15
	DppMsgTypeUSBWWID                     //16
	DppMsgTypeDevLU                       //17
	DppMsgTypeSATA                        //18
	DppMsgTypeISCSI                       //19
	DppMsgTypeVLAN                        //20
	_                                     //21
	DppMsgTypeSASEx                       //22
	DppMsgTypeNVME                        //23
	DppMsgTypeURI                         //24
	DppMsgTypeUFS                         //25
	DppMsgTypeSD                          //26
	DppMsgTypeBT                          //27
	DppMsgTypeWiFi                        //28
	DppMsgTypeeMMC                        //29
	DppMsgTypeBLE                         //30
	DppMsgTypeDNS                         //31
	DppMsgTypeNVDIMM                      //32
	DppMsgTypeRest                        //documented as 32, likely 33
)

func (e EfiDppMsgSubType) String() string {
	switch e {
	case DppMsgTypeATAPI:
		return "ATAPI"
	case DppMsgTypeSCSI:
		return "SCSI"
	case DppMsgTypeFibreCh:
		return "Fibre Channel"
	case DppMsgTypeFirewire:
		return "1394"
	case DppMsgTypeUSB:
		return "USB"
	case DppMsgTypeIIO:
		return "I20"
	case DppMsgTypeInfiniband:
		return "Infiniband"
	case DppMsgTypeVendor:
		return "Vendor"
	case DppMsgTypeMAC:
		return "MAC"
	case DppMsgTypeIP4:
		return "IPv4"
	case DppMsgTypeIP6:
		return "IPv6"
	case DppMsgTypeUART:
		return "UART"
	case DppMsgTypeUSBClass:
		return "USB Class"
	case DppMsgTypeUSBWWID:
		return "USB WWID"
	case DppMsgTypeDevLU:
		return "Device Logical Unit"
	case DppMsgTypeSATA:
		return "SATA"
	case DppMsgTypeISCSI:
		return "iSCSI"
	case DppMsgTypeVLAN:
		return "VLAN"
	case DppMsgTypeSASEx:
		return "SAS Ex"
	case DppMsgTypeNVME:
		return "NVME"
	case DppMsgTypeURI:
		return "URI"
	case DppMsgTypeUFS:
		return "UFS"
	case DppMsgTypeSD:
		return "SD"
	case DppMsgTypeBT:
		return "Bluetooth"
	case DppMsgTypeWiFi:
		return "WiFi"
	case DppMsgTypeeMMC:
		return "eMMC"
	case DppMsgTypeBLE:
		return "BLE"
	case DppMsgTypeDNS:
		return "DNS"
	case DppMsgTypeNVDIMM:
		return "NVDIMM"
	case DppMsgTypeRest:
		return "REST"
	default:
		return fmt.Sprintf("UNKNOWN-0x%x", uint8(e))
	}
}

//pg 293
type DppMsgATAPI struct {
	Hdr             EfiDevicePathProtocolHdr
	Primary, Master bool
	LUN             uint16
}

var _ EfiDevicePathProtocol = (*DppMsgATAPI)(nil)

func ParseDppMsgATAPI(h EfiDevicePathProtocolHdr, b []byte) (*DppMsgATAPI, error) {
	if h.Length != 8 {
		return nil, EParse
	}
	msg := &DppMsgATAPI{
		Hdr:     h,
		Primary: b[0] == 0,
		Master:  b[1] == 0,
		LUN:     binary.LittleEndian.Uint16(b[2:4]),
	}
	return msg, nil
}

func (e *DppMsgATAPI) Header() EfiDevicePathProtocolHdr { return e.Hdr }
func (e *DppMsgATAPI) ProtoSubTypeStr() string {
	return EfiDppMsgSubType(e.Hdr.ProtoSubType).String()
}
func (e *DppMsgATAPI) String() string {
	return fmt.Sprintf("ATAPI(pri=%t,master=%t,lun=%d)", e.Primary, e.Master, e.LUN)
}

func (e *DppMsgATAPI) Resolver() (EfiPathSegmentResolver, error) {
	return nil, EUnimpl
}

//pg 300
type DppMsgMAC struct {
	Hdr    EfiDevicePathProtocolHdr
	Mac    [32]byte //0-padded
	IfType uint8    //RFC3232; seems ethernet is 6
}

func ParseDppMsgMAC(h EfiDevicePathProtocolHdr, b []byte) (*DppMsgMAC, error) {
	if h.Length != 37 {
		return nil, EParse
	}
	mac := &DppMsgMAC{
		Hdr: h,
		//Mac:    b[:33],
		IfType: b[32],
	}
	copy(mac.Mac[:], b[:32])
	return mac, nil
}

func (e *DppMsgMAC) Header() EfiDevicePathProtocolHdr { return e.Hdr }

func (e *DppMsgMAC) ProtoSubTypeStr() string {
	return EfiDppMsgSubType(e.Hdr.ProtoSubType).String()
}

func (e *DppMsgMAC) String() string {
	switch e.IfType {
	case 1:
		return fmt.Sprintf("MAC(%x)", e.Mac[:6])
	default:
		return fmt.Sprintf("MAC(mac=%08x,iftype=0x%x)", e.Mac, e.IfType)
	}
}

func (e *DppMsgMAC) Resolver() (EfiPathSegmentResolver, error) {
	return nil, EUnimpl
}
