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

//functions in this file are what a user is most likely to need.

// Msg writes an ASCII message. It returns row,col of last character - same as
// used with Update() function. Do not use with non-ASCII chars, as they are
// likely to be converted to utf-8 and corrupted in the process.
func (l *Lcd) Msg(msg string) (Coord, error) {
	if l == nil {
		return Coord{}, nil
	}
	message := []byte(msg)
	mlen := len(message)
	if mlen == 0 {
		return Coord{}, nil
	}
	max := l.MaxCursorPos()
	l.mutex.Lock()
	defer l.mutex.Unlock()
	lines := fit(message, max)
	return l.writeLines(lines, max)
}

//Scroll a long message vertically. Non-ASCII chars will not render correctly.
func (l *Lcd) LongMsg(msg string, cycle, displayTime time.Duration) error {
	if l == nil {
		time.Sleep(displayTime)
		return ENil
	}
	done := make(chan struct{})
	go func() {
		time.Sleep(displayTime)
		close(done)
	}()
	return l.LongMsgUntil(done, msg, cycle)
}

func (l *Lcd) LongMsgUntil(done chan struct{}, msg string, cycle time.Duration) (err error) {
	if l == nil {
		<-done
		err = ENil
		return
	}
	l.mutex.Lock()
	defer l.mutex.Unlock()
	b := NewBlurb(l, LcdTxt(msg), Coord{0, 0}, Coord{Row: l.dims.Row, Col: l.Width()})
	_, err = b.draw(true)
	if err != nil && l.DbgGeneral {
		log.Logf("LongMsgUntil error: %s", err)
	}
	for {
		select {
		case <-done:
			return
		case <-time.After(cycle):
			_, err = b.draw(false)
			if err != nil && l.DbgGeneral {
				log.Logf("LongMsgUntil error: %s", err)
			}
		}
	}
}

type FadeStyle int

const (
	Flash FadeStyle = iota //abrupt - square wave
	Fade                   //gradual - sine wave
)

//Like LongMsg, but changes brightness to get attention
func (l *Lcd) BlinkMsg(msg string, fade FadeStyle, cycle, displayTime time.Duration) (err error) {
	done := make(chan struct{})
	go func() {
		<-time.After(displayTime)
		close(done)
	}()
	return l.BlinkMsgUntil(done, msg, fade, cycle)
}

//Like BlinkMsg but call from goroutine; displays until `done` is closed.
func (l *Lcd) BlinkMsgUntil(done chan struct{}, msg string, fade FadeStyle, cycle time.Duration) (err error) {
	if l == nil {
		<-done
		return ENil
	}
	l.mutex.Lock()
	defer l.mutex.Unlock()
	defer func() {
		e := l.defaultBacklight()
		if err == nil {
			err = e
		}
	}()
	blurb := NewBlurb(l, LcdTxt(msg), Coord{0, 0}, Coord{Row: l.dims.Row, Col: l.Width()})
	_, err = blurb.draw(true)
	if err != nil && l.DbgGeneral {
		log.Logf("BlinkMsgUntil error: %s", err)
	}
	switch fade {
	case Flash:
		var b uint8 = 90
		for {
			select {
			case <-done:
				return
			default:
			}
			_, err = blurb.draw(false)
			if err != nil && l.DbgGeneral {
				log.Logf("BlinkMsgUntil error: %s", err)
			}
			err = l.toggleBacklight(&b)
			if err != nil && l.DbgGeneral {
				log.Logf("BlinkMsgUntil error: %s", err)
			}
			time.Sleep(cycle)
		}
	case Fade:
		var nrSteps uint = 20
		var s uint = 0
		for {
			select {
			case <-done:
				return
			default:
			}
			_, err = blurb.draw(false)
			if err != nil && l.DbgGeneral {
				log.Logf("BlinkMsgUntil error: %s", err)
			}
			var x uint
			for x = 0; x < nrSteps; x++ {
				err = l.backlightSinStep(&s, nrSteps)
				if err != nil && l.DbgGeneral {
					log.Logf("BlinkMsgUntil error: %s", err)
				}
				time.Sleep(cycle / time.Duration(nrSteps))
			}
		}
	}
	return
}

//display message with timeout, return true if a button was pressed during that time
//TODO display remaining time on screen?
func (l *Lcd) PressAnyKey(desc string, cycle, timeout time.Duration) (pressed bool, err error) {
	if l == nil {
		time.Sleep(timeout)
		err = ENil
		return
	}
	done := make(chan struct{})
	go func() {
		e := l.LongMsgUntil(done, "Press any key to interrupt "+desc, cycle)
		if e != nil && l.DbgGeneral {
			log.Logf("PressAnyKey error: %s", e)
			err = e
		}
	}()
	key := l.WaitForEvent(timeout)
	close(done)
	err = l.Clear() //locks mutex, which has side effect of ensuring LongMsgUntil has exited
	if err != nil && l.DbgGeneral {
		log.Logf("PressAnyKey error: %s", err)
	}
	err = l.defaultBacklight()
	if err != nil && l.DbgGeneral {
		log.Logf("PressAnyKey error: %s", err)
	}
	if key != KEY_NO_KEY {
		err = l.write(Coord{}, LcdTxt("interrupted"), true)
		if err != nil {
			return
		}
		pressed = true
	} else {
		err = l.write(Coord{}, LcdTxt("timeout"), true)
		if err != nil {
			return
		}
	}
	return
}

//Same as PressAnyKey, but wait for a channel instead of using a timeout.
func (l *Lcd) PressAnyKeyUntil(desc string, cycle time.Duration, done chan struct{}) (pressed bool, err error) {
	if l == nil {
		<-done
		err = ENil
		return
	}
	//we use a 2nd channel so we can close immediately upon reciept of event.
	//otherwise, LongMsgUntil and other commands are fighting for control of
	//display. cannot close(done) because caller does and closing an
	//already-closed channel causes panic.
	internalDone := make(chan struct{})
	go func() {
		e := l.LongMsgUntil(internalDone, "Press any key to interrupt "+desc, cycle)
		if e != nil && l.DbgGeneral {
			log.Logf("PressAnyKeyUntil error: %s", e)
			err = e
		}
	}()
	key := l.waitForEvent(DoneChToTimeCh(done))
	close(internalDone)
	err = l.Clear()
	if err != nil && l.DbgGeneral {
		log.Logf("PressAnyKeyUntil error: %s", err)
	}

	err = l.defaultBacklight()
	if err != nil && l.DbgGeneral {
		log.Logf("PressAnyKeyUntil error: %s", err)
	}
	if key != KEY_NO_KEY {
		err = l.write(Coord{}, LcdTxt("interrupted"), true)
		pressed = true
	} else {
		err = l.write(Coord{}, LcdTxt("timeout"), true)
	}
	return
}

//Convert done (chan struct{}) to a Time channel. For use with l.waitForEvent.
func DoneChToTimeCh(done chan struct{}) <-chan time.Time {
	tc := make(chan time.Time)
	go func() {
		<-done
		close(tc)
	}()
	return tc
}
