// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package cfa

import (
	"fmt"
	"gprovision/pkg/log"
	"os"
	"sync"
)

type Model int

const (
	Cfa631 Model = iota
	Cfa635
)

//lcd state that we should preserve - key press/release masks, backlight
//could also preserve contrast but we do not change that
type storableState struct {
	pressMask, releaseMask KeyMask
	backlight              byte
	ignoreState            bool //set to not write state back to unit
}

type Lcd struct {
	dev         *SerialDev
	dims        Coord
	model       Model
	mutex       sync.Mutex    //exposed functions should always lock if they send commands
	legend      Legend        //keep track of current key legend
	legendWidth byte          //number of bytes taken up by current legend
	prevState   storableState //lcd state that can be read/written
	DbgGeneral  bool          //log info to aid in debugging
	DbgMenu     bool          //debug menus
}

//Restore mutable state to lcd and close port.
func (l *Lcd) Close() error {
	if l == nil || l.dev == nil {
		return nil
	}
	l.mutex.Lock()
	defer l.mutex.Unlock()
	err := l.restoreState()
	if err != nil {
		return err
	}
	err = l.dev.Close()
	l.dev = nil
	return err
}

//return max cursor position (add 1 to X and Y for screen dims)
func (l *Lcd) MaxCursorPos() Coord { return l.dims }

//returns raw model/revision string from device
func (l *Lcd) Revision() (info string, err error) {
	if l == nil {
		return
	}
	l.mutex.Lock()
	defer l.mutex.Unlock()
	var b []byte
	_, b, err = l.sendCmd(Cmd_HwFwVers, []byte{})
	if err == nil {
		info = string(b)
	}
	return
}

func (l *Lcd) captureState() (err error) {
	_, rData, err := l.sendCmd(Cmd_ReadReprtStat, nil)
	if err != nil {
		log.Logf("getting lcd state: err %s, data %q", err, rData)
		l.prevState.ignoreState = true
		return
	}
	if len(rData) != 15 {
		log.Logf("unable to record lcd state, expected 15 bytes but got %d: [% 2x]", len(rData), rData)
		err = os.ErrInvalid
		l.prevState.ignoreState = true
		return
	}
	l.prevState.pressMask = KeyMask(rData[5])
	l.prevState.releaseMask = KeyMask(rData[6])
	l.prevState.backlight = rData[14]
	return
}

func (l *Lcd) restoreState() (err error) {
	if l.prevState.ignoreState {
		return
	}
	var rData []byte
	_, rData, err = l.sendCmd(Cmd_CfgKeyReports, []byte{
		byte(l.prevState.pressMask),
		byte(l.prevState.releaseMask),
	})
	if err != nil {
		log.Logf("restoring key reporting: data %v, err %s\n", rData, err)
	}
	_, rData, err = l.sendCmd(Cmd_SetBacklight, []byte{l.prevState.backlight})
	if err != nil {
		log.Logf("restoring backlight: data %v, err %s\n", rData, err)
	}
	return
}

//returns detected device model
func (l *Lcd) Model() Model { return l.model }

func (l *Lcd) sendCmd(cmd Command, data []byte) (rCmd Command, rData []byte, err error) {
	p := &pktNoCrc{}
	err = p.SetCommand(cmd)
	if err != nil {
		return
	}
	err = p.SetData(data)
	if err != nil {
		return
	}
	var resp *Packet
	resp, err = l.dev.sendPktRetry(p)
	if err == nil && resp != nil {
		rData = resp.Data()
		rCmd = resp.Cmd()
	}
	return
}

func (l *Lcd) Ping() (bool, error) {
	if l == nil {
		return false, EMissing
	}
	l.mutex.Lock()
	defer l.mutex.Unlock()
	return l.dev.ping()
}

type Coord struct{ Col, Row byte }

var ELen = fmt.Errorf("String length out of range")
var ERange = fmt.Errorf("Coordinate(s) out of range")
var EFit = fmt.Errorf("Will not fit on display")
var ENil = fmt.Errorf("Lcd is nil")

// Characters to be displayed on the screen. Generally used for text that fits
// on a single line. Must use byte arrays rather than strings, as non-ascii
// characters will otherwise get translated into utf-8 before the lcd sees them,
// resulting in gibberish.
type LcdTxt []byte

//Write text to the screen beginning at 'start'
func (l *Lcd) Write(start Coord, txt LcdTxt) (err error) {
	if l == nil {
		return
	}
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if l.DbgGeneral {
		log.Logf("at %v: write '%s'", start, txt)
	}
	return l.write(start, txt, false)
}

//writes to lcd, ensuring data will fit on lcd
func (l *Lcd) write(start Coord, txt LcdTxt, clear bool) (err error) {
	if byte(len(txt))+start.Col > l.Width() {
		return ELen
	}
	if start.Col > l.dims.Col || start.Row > l.dims.Row {
		return ERange
	}
	var blanks byte
	if clear {
		blanks = l.Width() - start.Col - byte(len(txt))
	}
	data := make([]byte, len(txt)+2+int(blanks))
	data[0], data[1] = start.Col, start.Row
	copy(data[2:], txt)
	for i := range data[2+len(txt):] {
		data[2+len(txt)+i] = 0x20
	}
	_, _, err = l.sendCmd(Cmd_WriteDisp, data)
	return
}

//writes to lcd. ignores Width().
func (l *Lcd) writeByte(start Coord, b byte) (err error) {
	if start.Col > l.dims.Col || start.Row > l.dims.Row {
		return ERange
	}
	_, _, err = l.sendCmd(Cmd_WriteDisp, []byte{start.Col, start.Row, b})
	return
}

//Clear screen and put cursor at 0,0
func (l *Lcd) Clear() error {
	if l == nil {
		return nil
	}
	l.mutex.Lock()
	defer l.mutex.Unlock()
	return l.clear()
}
func (l *Lcd) clear() error {
	_, _, err := l.sendCmd(Cmd_Clear, []byte{})
	return err
}

func (l *Lcd) SetCursorStyle(s byte) {
	if l == nil {
		return
	}
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.setCursorStyle(s)
}
func (l *Lcd) setCursorStyle(s byte) {
	if s > 4 {
		log.Logf("cursor style %d is out of range", s)
		return
	}
	_, _, err := l.sendCmd(Cmd_SetCursorStyle, []byte{s})
	if err != nil && l.DbgGeneral {
		log.Logf("setting cursor style: %s", err)
	}
}

func (l *Lcd) HideCursor() {
	if l == nil {
		return
	}
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.hideCursor()
}
func (l *Lcd) hideCursor() {
	l.setCursorStyle(0)
}

func (l *Lcd) SetCursorPosition(p Coord) {
	if l == nil {
		return
	}
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.setCursorPosition(p)
}
func (l *Lcd) setCursorPosition(p Coord) {
	if p.Col > l.dims.Col || p.Row > l.dims.Row {
		log.Logf("cursor pos %v out of range; must be less than %v", p, l.dims)
		return
	}
	_, _, err := l.sendCmd(Cmd_SetCursorPos, []byte{p.Col, p.Row})
	if err != nil && l.DbgGeneral {
		log.Logf("setting cursor pos: %s", err)
	}
}

//true if txt will fit on screen
func (l *Lcd) Fits(txt LcdTxt) bool {
	if l == nil {
		return false
	}
	return len(txt) <= int(l.Width())
}

const (
	offScreen byte = 255
)

//return number of usable chars on a line, taking into account overlay
func (l *Lcd) Width() byte {
	if l == nil {
		return 0
	}
	return l.dims.Col - l.legendWidth + 1
}

//Poll lcd for key activity. After a short delay, results will be available to Event() or WaitForEvent().
func (l *Lcd) PollKeys() error {
	if l == nil {
		return nil
	}
	l.mutex.Lock()
	defer l.mutex.Unlock()
	return l.pollKeys()
}

//Poll lcd for key activity. Sends command to LCD, but does _not_ wait for
//response - response is intercepted by handleIncoming() and data goes into
//the Event channel.
func (l *Lcd) pollKeys() error {
	p := &pktNoCrc{
		command: Cmd_ReadKeysPolled,
	}
	return l.dev.sendOnly(p)
}
