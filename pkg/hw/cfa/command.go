// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package cfa

import (
	"fmt"
)

/* A command in a transmitted packet or a response code in a received packet.
When uncommenting additional commands, remember to add them to the String()
method.
*/
type Command byte

const (
	//Commands sent to LCD
	Cmd_Ping     Command = iota //0x00
	Cmd_HwFwVers                //0x01

	//underscore: iota still increments. Useful for unused values.
	_                      // Cmd_WriteUserFlash     //0x02
	Cmd_ReadUserFlash      //0x03
	_                      // Cmd_StoreBootState     //0x04
	Cmd_Reboot             //0x05
	Cmd_Clear              //0x06
	_                      // Cmd_SetLine1_CFA633    //0x07
	_                      // Cmd_SetLine2_CFA633    //0x08
	_                      // Cmd_SetCGROM           //0x09
	_                      // Cmd_Read8              //0x0a
	Cmd_SetCursorPos       //0x0b
	Cmd_SetCursorStyle     //0x0c
	_                      // Cmd_SetContrast        //0x0d
	Cmd_SetBacklight       //0x0e
	_                      // Cmd_Undefined_0x0F     //0x0f
	_                      // Cmd_Scab_FanRpt        //0x10
	_                      // Cmd_Scab_FanPwr        //0x11
	_                      // Cmd_Scab_TmpRead       //0x12
	_                      // Cmd_Scab_TmpRpt        //0x13
	_                      // Cmd_Scab_Arbitrary     //0x14
	_                      // Cmd_Scab_Disp          //0x15
	_                      // Cmd_Direct_Disp        //0x16
	Cmd_CfgKeyReports      //0x17
	Cmd_ReadKeysPolled     //0x18
	_                      // Cmd_Scab_SetFailsafe       //0x19
	_                      // Cmd_Scab_SetTachGlitch     //0x1a
	_                      // Cmd_Scab_QueryFanFailsafe  //0x1b
	_                      // Cmd_Atx_SetSwFunc          //0x1c
	_                      // Cmd_Watchdog               //0x1d
	Cmd_ReadReprtStat      //0x1e
	Cmd_WriteDisp          //0x1f
	Cmd_KeyLegendOnOffMask //0x20
	_                      // Cmd_SetBaud                //0x21
	_                      // Cmd_Scab_CfgGpio           //0x22
	_                      // Cmd_Scab_ReadGpio          //0x23

	//Responses from the LCD have a command that is one of the above + 0x40

	//Reports from the LCD
	Report_Key = 0x80
	// Report_FanSpeed = 0x81
	// Report_Temps    = 0x82
)

//Masks the packet type bytes
const CmdMask Command = 0x3f

func (c Command) CommandFromResponse() Command {
	return c & CmdMask
}
func (c Command) Type() PktType {
	return PktType(c >> 6)
}

const (
	LowestErrVal    Command = 0xC0
	LowestReportVal         = 0x80
	LowestOKVal             = 0x40
)

//Returns a string representing the command, with no whitespace. Lack of
//whitespace seems to make logs slightly easier to read. A 4-character prefix
//indicates the class of command (actual command, ok response, report, error).
func (c Command) String() string {

	pfx := "CMD_" //default prefix
	/* low bits in error and OK responses match the original command, so
	in these cases change the prefix and filter the high bits out of c */
	original := c
	switch {
	case c >= LowestErrVal:
		pfx = "ERR_"
		c = c.CommandFromResponse()
	case c >= LowestReportVal:
		if c == Report_Key {
			return "RPT_KeyActivity"
		}
		//all other reports render as something unknown
		pfx = "RPT_"
	case c >= LowestOKVal:
		pfx = "OK__"
		c = c.CommandFromResponse()
	}

	var s string
	switch c {
	case Cmd_Ping:
		s = "Ping"
	case Cmd_HwFwVers:
		s = "HwFwVers"
	case Cmd_ReadUserFlash:
		s = "ReadUserFlash"
	case Cmd_Reboot:
		s = "Reboot"
	case Cmd_Clear:
		s = "Clear"
	case Cmd_SetCursorPos:
		s = "SetCursorPos"
	case Cmd_SetCursorStyle:
		s = "SetCursorStyle"
	case Cmd_SetBacklight:
		s = "SetBacklight"
	case Cmd_CfgKeyReports:
		s = "CfgKeyReports"
	case Cmd_ReadKeysPolled:
		s = "ReadKeys"
	case Cmd_ReadReprtStat:
		s = "ReadReportingAndStatus"
	case Cmd_WriteDisp:
		s = "Write"
	case Cmd_KeyLegendOnOffMask:
		s = "SetKeyLegend"
	default:
		s = fmt.Sprintf("UNKNOWN_0x%02x", int(original))
	}
	return pfx + s
}

//indexes into data returned for Cmd_ReadKeysPolled
const (
	KeyPollCurrent int = iota
	KeyPollPressed
	KeyPollReleased
)
