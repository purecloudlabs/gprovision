// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package cfa

import (
	"math"
)

/* backlight functions
 * uses hand picked values; display brightness is non-linear
 * low values like 5 --> display off
 * values higher than 50 result in almost identical brightness
 */

func (l *Lcd) SetBacklight(bright uint8) error {
	if l == nil {
		return nil
	}
	l.mutex.Lock()
	defer l.mutex.Unlock()
	return l.setBacklight(bright)
}
func (l *Lcd) setBacklight(bright uint8) (err error) {
	_, _, err = l.sendCmd(Cmd_SetBacklight, []byte{bright})
	return
}

//bright - dim - bright - dim ...
func (l *Lcd) toggleBacklight(b *uint8) error {
	*b = 100 - *b
	return l.setBacklight(*b)
}

//set brightness via sine wave
func (l *Lcd) backlightSinStep(curStep *uint, nrSteps uint) error {
	if *curStep >= nrSteps {
		*curStep = 0
	}
	multiplier := 20.0
	offset := 8.0
	if l.model == Cfa635 {
		multiplier = 100.0
		offset = 0.0
	}
	b := math.Sin(math.Pi*float64(*curStep)/float64(nrSteps))*multiplier + offset
	*curStep++
	return l.setBacklight(uint8(b))
}

func (l *Lcd) DefaultBacklight() error {
	if l == nil {
		return nil
	}
	l.mutex.Lock()
	defer l.mutex.Unlock()
	return l.defaultBacklight()
}
func (l *Lcd) defaultBacklight() error {
	return l.setBacklight(l.prevState.backlight)
}
