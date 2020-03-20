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

/*
Menu creates a menu with the given items, one per line, navigable using the LCD
buttons. Vertical scrolling is allowed when the number of items exceeds the
LCD's height. When an item's length exceeds display width, that item
automatically scrolls horizontally - with a pause at beginning and end.

If non-negative, the return value indicates the index of the item selected by
the user. Two negative values are also possible: CHOICE_NONE (-1) if no choice
is made within the timeout, or CHOICE_CANCEL (-2) if the user presses the
cancel button.

On the CFA-631, the legend is enabled to indicate to the user what the buttons
do. The width of this legend is taken into account for rendering and scrolling.

TODO: add symbol to cursor?
*/
func (l *Lcd) Menu(items []LcdTxt, timeout time.Duration, keyPolling bool) Choice {
	updateTicker := NewTicker(time.Second / 10)
	scrollTicker := NewTicker(time.Second)
	done := make(chan struct{})
	go func() {
		time.Sleep(timeout)
		close(done)
	}()
	return l.menuWithTicks(items, done, updateTicker, scrollTicker, keyPolling, nil)
}

//updateTicker and scrollTicker *must not* be the same ticker, though they can have the same period
func (l *Lcd) menuWithTicks(items []LcdTxt, done chan struct{}, updateTicker, scrollTicker *Ticker, keyPolling bool, syncTick chan<- time.Time) Choice {
	if l == nil {
		<-done
		return CHOICE_NONE
	}
	l.mutex.Lock()
	defer l.mutex.Unlock()

	defer func() {
		_, _, err := l.sendCmd(Cmd_Clear, []byte{})
		if err != nil && l.DbgMenu {
			log.Logf("menuWithTicks: clear: %s", err)
		}
	}()

	l.setCursorStyle(1)
	defer l.hideCursor()

	if keyPolling {
		l.setupKeyReporting(false, false)
	} else {
		l.setupKeyReporting(false, true)
	}

	//enable legend overlay (no effect except on 631)
	l.setLegend(Legend_VDX, true)
	defer l.setLegend(LegendNone, true)

	defer updateTicker.Stop()
	defer scrollTicker.Stop()

	m := l.createMenu(items, done, updateTicker.C, scrollTicker.C, keyPolling)
	m.syncTick = syncTick
	defer m.scrollTickDistrib.Stop()
	return m.Run()
}
func (l *Lcd) createMenu(items []LcdTxt, done chan struct{}, updateTick, scrollTick <-chan time.Time, keyPolling bool) *menu {
	m := &menu{
		done: done,
		v: view{
			choice:       CHOICE_NONE,
			redrawLines:  true, //ensure it gets drawn first time through
			redrawCursor: true,
			height:       int(l.dims.Row),
			max:          len(items) - 1,
			l:            l,
		},
		l:          l,
		keyPolling: keyPolling,
		updateTick: updateTick,
	}
	m.scrollTickDistrib = NewTickDistrib(scrollTick, len(items))
	for i, it := range items {
		b := NewBlurb(l, it,
			Coord{},
			Coord{Col: l.Width()})
		b.setTick(m.scrollTickDistrib.Get(uint(i)))
		m.items = append(m.items, b)
	}
	return m
}

type menu struct {
	done              chan struct{}
	items             []blurb
	l                 *Lcd
	v                 view
	keyPolling        bool
	startCol          byte
	updateTick        <-chan time.Time
	scrollTickDistrib *TickDistrib
	syncTick          chan<- time.Time
}

//User selection from menu. Non-negative values correspond to menu item indexes.
type Choice int

const (
	CHOICE_NONE   Choice = -1 - iota
	CHOICE_CANCEL        //-2
)

func (m *menu) Run() Choice {
	for m.v.choice == CHOICE_NONE {
		select {
		case <-m.done:
			log.Logf("menu timeout")
			return CHOICE_NONE
		default:
		}
		m.draw()
		if m.keyPolling {
			err := m.l.pollKeys()
			if err != nil && m.l.DbgMenu {
				log.Logf("menu key poll: %s", err)
			}
		}
		evt := m.l.waitForEvent(m.updateTick)
		m.v.update(evt)
	}
	return m.v.choice
}

//display items one per line. scroll long ones.
func (m *menu) draw() {
	if m.syncTick != nil {
		defer func() { m.syncTick <- time.Now() }()
	}
	if !m.v.redrawLines && !m.v.overflow && !m.v.redrawCursor {
		//nothing to do
		return
	}
	if m.l.DbgMenu {
		log.Logf("[D] draw")
	}
	//overflow gets recalculated every time, which is more often than necessary,
	// but fixing that would make the code uglier
	overflow := false
	for dispLine := byte(0); dispLine <= m.l.dims.Row; dispLine++ {
		//for each display line, draw an item
		idx := m.v.first + int(dispLine)
		if idx >= len(m.items) {
			continue
		}
		overflow = overflow || !m.items[idx].fits()
		if !m.v.redrawLines && m.items[idx].fits() {
			//no need to redraw
			continue
		}
		m.items[idx].move(Coord{Row: dispLine, Col: m.startCol})
		_, err := m.items[idx].draw(m.v.redrawLines)
		if err != nil {
			if m.l.DbgMenu {
				log.Logf("draw error: %s", err)
				log.Logf("v=%#v", m.v)
			}
			//force everything to redraw next time
			m.v.redrawLines = true
			m.v.redrawCursor = true
			return
		}
	}
	m.v.overflow = overflow

	if m.v.redrawCursor {
		//put cursor on selected line
		selLine := m.v.selected - m.v.first
		_, _, err := m.l.sendCmd(Cmd_SetCursorPos, []byte{0, byte(selLine)})
		if err != nil && m.l.DbgMenu {
			log.Logf("draw error: %s", err)
		}
	}

	//reset horiz scroll state for all off-screen items
	for i := range m.items {
		if m.v.offscreen(i) {
			m.items[i].move(Coord{Row: offScreen, Col: m.startCol})
		}
	}
}

//used for menu to track on-screen items and to signal a choice
type view struct {
	first        int    //index of item on first row
	selected     int    //index of item selected. always on screen.
	choice       Choice //index of chosen item, -1 for timeout/no action, or -2 for user cancel
	max          int    //max index / last item
	overflow     bool   //if true, indicates at least one item too large to fit horizontally
	height       int    //number of lines on lcd
	redrawLines  bool   //true if displayed items changed
	redrawCursor bool   //true if screen line of cursor changed
	l            *Lcd
}

//update view based upon events. for cfa631, only makes sense for overlay OverlayUVDX
//does nothing for key presses, only releases
func (v *view) update(k KeyActivity) {
	v.redrawLines = false
	v.redrawCursor = false
	switch k {
	case KEY_UL_RELEASE:
		fallthrough
	case KEY_UP_RELEASE:
		//up
		if v.selected > 0 {
			v.selected--
			if v.selected < v.first {
				v.first--
				//visible items change but cursor remains in same place
				v.redrawLines = true
			} else {
				//visible items do not change, cursor needs to move though
				v.redrawCursor = true
			}
			if v.selected == 0 {
				v.l.setLegend(Legend_VDX, true)
			} else {
				v.l.setLegend(LegendUVDX, true)
			}
		}
	case KEY_LL_RELEASE:
		fallthrough
	case KEY_DOWN_RELEASE:
		//down
		if v.selected < v.max {
			v.selected++
			if v.first+v.height < v.selected {
				v.first++
				//visible items change but cursor remains in same place
				v.redrawLines = true
			} else {
				//visible items do not change, cursor needs to move though
				v.redrawCursor = true
			}
			if v.selected == v.max {
				v.l.setLegend(LegendUV_X, true)
			} else {
				v.l.setLegend(LegendUVDX, true)
			}
		}
	case KEY_UR_RELEASE:
		fallthrough
	case KEY_ENTER_RELEASE:
		//enter
		v.choice = Choice(v.selected)
	case KEY_LR_RELEASE:
		fallthrough
	case KEY_EXIT_RELEASE:
		//cancel
		v.choice = CHOICE_CANCEL
	default:
		//ignore other keys
		if k != KEY_NO_KEY && v.l.DbgMenu {
			log.Logf("[D] unknown key event 0x%02x", k)
		}
	}
	if v.l.DbgMenu {
		log.Logf("[D] update: first=%d, selection=%d, rl=%t, rc=%t", v.first, v.selected, v.redrawLines, v.redrawCursor)
	}
}

//return true if items[idx] is offscreen
func (v *view) offscreen(idx int) bool {
	return idx < v.first || idx > v.first+v.height
}
