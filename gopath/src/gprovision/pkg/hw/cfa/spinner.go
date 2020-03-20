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

//Spinner is a message + ASCII progress spinner. Start with Display(), update
//with spinner.Next().
//
//NOTE - will not prevent, detect, or recover gracefully from something else
//changing the display.
type Spinner struct {
	Msg      string
	seq      int
	loc, max Coord
	last     time.Time
	Lcd      *Lcd
}

var spinnerSprites []byte

func init() {
	/* chars (x is approx):  |     /    -    +    *    x     \     */
	spinnerSprites = []byte{0xfe, '/', '-', '+', '*', 0x24, 0xfb}
}

//clear display and show s.Msg
//also calculate coords for spinner
//put a space between message and spinner when possible
func (s *Spinner) Display() error {
	if s.Lcd == nil {
		log.Logf("unable to construct spinner with nil Lcd")
		return ENil
	}
	var err error
	s.max = s.Lcd.MaxCursorPos()
	s.loc, err = s.Lcd.Msg(s.Msg)
	if err != nil {
		return err
	}
	s.loc.Col += 2
	if s.loc.Col > s.max.Col {
		if s.loc.Row < s.max.Row {
			s.loc.Row++
			s.loc.Col = 0
		} else {
			s.loc.Col = s.max.Col
		}
	}
	return nil
}

//Advance spinner to next state. Rate limited to < 1/s
func (s *Spinner) Next() {
	if s.Lcd == nil {
		return
	}
	if s.last.Add(time.Second).After(time.Now()) {
		return
	}
	if s.seq >= len(spinnerSprites) {
		s.seq = 0
	}
	err := s.Lcd.write(s.loc, []byte{spinnerSprites[s.seq]}, false)
	if err != nil && s.Lcd.DbgGeneral {
		log.Logf("Spinner error: %s", err)
	}

	s.seq++
	s.last = time.Now()
}

//Displays spinner until 'done' is closed. Interval must be at least 1s.
func (l *Lcd) SpinnerUntil(msg string, interval time.Duration, done chan struct{}) (err error) {
	if l == nil {
		<-done
		return
	}
	sp := Spinner{
		Msg: msg,
		Lcd: l,
	}
	err = sp.Display()
	l.mutex.Lock()
	defer l.mutex.Unlock()
	for {
		select {
		case <-done:
			return
		case <-time.After(interval):
			if err == nil {
				sp.Next()
			}
		}
	}
}
