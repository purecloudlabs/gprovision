// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package cfa

import (
	"gprovision/pkg/log"
	"time"
)

type KeyActivity byte

const (
	KEY_NO_KEY KeyActivity = iota
	/* CFA-635 (external: XES635) */
	KEY_UP_PRESS //1
	KEY_DOWN_PRESS
	KEY_LEFT_PRESS
	KEY_RIGHT_PRESS
	KEY_ENTER_PRESS
	KEY_EXIT_PRESS
	KEY_UP_RELEASE
	KEY_DOWN_RELEASE
	KEY_LEFT_RELEASE
	KEY_RIGHT_RELEASE
	KEY_ENTER_RELEASE
	KEY_EXIT_RELEASE //12
	/* CFA-631 */
	KEY_UL_PRESS //13
	KEY_UR_PRESS
	KEY_LL_PRESS
	KEY_LR_PRESS
	KEY_UL_RELEASE
	KEY_UR_RELEASE
	KEY_LL_RELEASE
	KEY_LR_RELEASE //20
)

type KeyMask byte

//CFA 631: 4 buttons
const (
	KP_UL KeyMask = 1 << iota
	KP_UR
	KP_LL
	KP_LR
	KP_ALL_631 = KP_UL | KP_UR | KP_LL | KP_LR
)

//CFA 635: 6 buttons
const (
	KP_UP    KeyMask = 1 << iota
	KP_ENTER         //check mark
	KP_EXIT          //CF docs call this KP_CANCEL. renamed for consistency.
	KP_LEFT
	KP_RIGHT
	KP_DOWN
	KP_UVDX_635 = KP_UP | KP_DOWN | KP_ENTER | KP_EXIT
	KP_LVRX_635 = KP_ENTER | KP_EXIT | KP_LEFT | KP_RIGHT
	KP_ALL_635  = KP_UVDX_635 | KP_LEFT | KP_RIGHT
)

//Return an event, or return nothing (KEY_NO_KEY) if maxWait exceeded.
func (l *Lcd) WaitForEvent(maxWait time.Duration) (ka KeyActivity) {
	if l == nil {
		time.Sleep(maxWait)
		return
	}
	//uses channel - no need for mutex
	return l.waitForEvent(time.After(maxWait))
}
func (l *Lcd) waitForEvent(ch <-chan time.Time) (ka KeyActivity) {
	select {
	case ka = <-l.dev.Events:
	case <-ch:
	}
	for len(ch) > 0 {
		<-ch //throw away outstanding ticks, don't get blocked
	}
	return
}

//Return an event if one has occurred. Short delay.
func (l *Lcd) Event() (ka KeyActivity) {
	//uses channel - no need for mutex
	return l.WaitForEvent(time.Millisecond) //only enough delay that we find an event, if there is one, before the delay expires
}

//Clear any outstanding events
func (l *Lcd) FlushEvents() {
	for len(l.dev.Events) > 0 {
		<-l.dev.Events
	}
}

//configure reporting of key press and release events
func (l *Lcd) SetupKeyReporting(press, release bool) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.setupKeyReporting(press, release)
}
func (l *Lcd) setupKeyReporting(press, release bool) {
	mask := byte(KP_ALL_631)
	if l.model == Cfa635 {
		mask = byte(KP_ALL_635)
	}
	data := []byte{0, 0}
	if press {
		data[0] = mask
	}
	if release {
		data[1] = mask
	}
	rCmd, rData, err := l.sendCmd(Cmd_CfgKeyReports, data)
	if err != nil || rCmd.Type() != CFResponse || rCmd.CommandFromResponse() != Cmd_CfgKeyReports {
		log.Logf("model %x, response %x, data %v, err %s\n", l.model, rCmd, rData, err)
	}
}

//Type of legend (only has an effect on cfa631)
type Legend int

const (
	//V = check mark, X = cancel/exit
	LegendNone Legend = iota //legend is disabled
	LegendUVDX               //top:    up, check; bottom:  down, X
	Legend_VDX               //top: blank, check; bottom:  down, X
	LegendUV_X               //top:    up, check; bottom: blank, X
	LegendLVRX               //top:  left, check; bottom: right, X
	Legend_VRX               //top: blank, check; bottom: right, X
	LegendLV_X               //top:  left, check; bottom: blank, X
)

//Codes for legend symbols for overlay
type LegendSymbol byte

const (
	LegendSymBlank LegendSymbol = iota
	LegendSymExit               //AKA Cancel or 'X'
	LegendSymCheck
	LegendSymUp
	LegendSymDown
	LegendSymRight
	LegendSymLeft
	LegendSymPlus
	LegendSymMinus
	LegendSymNone
)

//Only for CFA631
type LegendValues struct {
	Enable         bool
	UL, UR, LL, LR LegendSymbol
}

func (l Legend) Values() (lv LegendValues) {
	switch l {
	case LegendUVDX:
		lv.Enable = true
		lv.UL = LegendSymUp
		lv.UR = LegendSymCheck
		lv.LL = LegendSymDown
		lv.LR = LegendSymExit
	case Legend_VDX:
		lv.Enable = true
		lv.UL = LegendSymBlank
		lv.UR = LegendSymCheck
		lv.LL = LegendSymDown
		lv.LR = LegendSymExit
	case LegendUV_X:
		lv.Enable = true
		lv.UL = LegendSymUp
		lv.UR = LegendSymCheck
		lv.LL = LegendSymBlank
		lv.LR = LegendSymExit
	case LegendLVRX:
		lv.Enable = true
		lv.UL = LegendSymLeft
		lv.UR = LegendSymCheck
		lv.LL = LegendSymRight
		lv.LR = LegendSymExit
	case Legend_VRX:
		lv.Enable = true
		lv.UL = LegendSymBlank
		lv.UR = LegendSymCheck
		lv.LL = LegendSymRight
		lv.LR = LegendSymExit
	case LegendLV_X:
		lv.Enable = true
		lv.UL = LegendSymLeft
		lv.UR = LegendSymCheck
		lv.LL = LegendSymBlank
		lv.LR = LegendSymExit
	default:
		log.Logf("unknown legend type %d, disabling legend", l)
		fallthrough
	case LegendNone:
		//lv is initialized to zeros - no need to do anything
	}
	return
}

func (lv LegendValues) Bytes() []byte {
	if !lv.Enable {
		return []byte{0}
	}
	b := make([]byte, 5)
	b[0] = 1
	b[1] = byte(lv.UL)
	b[2] = byte(lv.UR)
	b[3] = byte(lv.LL)
	b[4] = byte(lv.LR)
	return b
}

const (
	legendWidth_631 = 3 //6 right-most chars according to pg58 of pdf. 3 per line
	legendWidth_635 = 1 //4 right-most chars, 1 per line
)

//symbols for display on LCD
const (
	SymLastCGRAM byte = iota + 0x0f

	SymRight      //triangular right arrow
	SymLeft       //triangular left arrow
	SymDoubleUp   //double up arrow
	SymDoubleDown //double down arrow
	SymLtLt       //double less-than (quotation mark in some languages)
	SymGtGt       //double greater-than (quotation mark in some langs)
	SymULArr      //diagonal arrow upper left
	SymURArr      //diagonal arrow upper right
	SymLLArr      //diagonal arrow lower left
	SymLRArr      //diagonal arrow lower right
	SymUp         //triangular up arrow
	SymDown       //triangular down arrow
	SymEnter      //enter key arrow symbol
	SymCaret      //caret, aka circumflex
	SymCaron      //upside-down circumflex
	SymFilled     //all pixels on
	SymSpace      //all pixels off
)

//Set up or clear overlay (native on 631). If also635 is true, draws arrow(s)
//in rightmost corner(s) if up/down is enabled. Does not draw left/right for 635.
func (l *Lcd) setLegend(legend Legend, also635 bool) {
	if l.legend == legend {
		return
	}
	switch l.model {
	default:
		l.legend = LegendNone
		l.legendWidth = 0
		return
	case Cfa631:
		l.legendWidth = legendWidth_631
		rCmd, rData, err := l.sendCmd(Cmd_KeyLegendOnOffMask, legend.Values().Bytes())
		if err != nil || rCmd.Type() != CFResponse || rCmd.CommandFromResponse() != Cmd_KeyLegendOnOffMask {
			log.Logf("model %x, response %x, data %v, err %s\n", l.model, rCmd, rData, err)
		}
	case Cfa635:
		if !also635 {
			l.legend = LegendNone
			l.legendWidth = 0
			return
		}
		//lcd does not do the heavy lifting itself, we need to do it
		var upchar, downchar byte
		l.legendWidth = legendWidth_635

		switch legend {
		case LegendUVDX:
			upchar = SymUp
			downchar = SymDown
		case LegendUV_X:
			upchar = SymUp
			downchar = SymCaron
		case Legend_VDX:
			upchar = SymCaret
			downchar = SymDown
		default:
			upchar = SymSpace
			downchar = SymSpace
			l.legendWidth = 0
		}
		err := l.writeByte(Coord{Row: 0, Col: 19}, upchar)
		if err != nil && l.DbgMenu {
			log.Logf("setLegend: %s", err)
		}
		err = l.writeByte(Coord{Row: 3, Col: 19}, downchar)
		if err != nil && l.DbgMenu {
			log.Logf("setLegend: %s", err)
		}
	}
	l.legend = legend
}

//for keymaskToActivity
type KeyXlate struct {
	m KeyMask
	a KeyActivity
}

var xlate635 = []KeyXlate{
	{KP_UP, KEY_UP_RELEASE},
	{KP_ENTER, KEY_ENTER_RELEASE},
	{KP_EXIT, KEY_EXIT_RELEASE},
	{KP_LEFT, KEY_LEFT_RELEASE},
	{KP_RIGHT, KEY_RIGHT_RELEASE},
	{KP_DOWN, KEY_DOWN_RELEASE},
}

var xlate631 = []KeyXlate{
	{KP_UL, KEY_UL_RELEASE},
	{KP_UR, KEY_UR_RELEASE},
	{KP_LL, KEY_LL_RELEASE},
	{KP_LR, KEY_LR_RELEASE},
}

//Translate from KeyMask to KeyActivity. Current data translates to release events.
//Used to translate poll responses to match non-poll events.
func keymaskToActivity(table []KeyXlate, m KeyMask) (ka []KeyActivity) {
	for _, entry := range table {
		if entry.m&m != 0 {
			ka = append(ka, entry.a)
		}
	}
	return
}
