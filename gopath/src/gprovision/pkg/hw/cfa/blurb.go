// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package cfa

import (
	"bytes"
	"gprovision/pkg/log"
	"time"
)

//stuff dealing with chunks of text that may not fit on a single line

//a chunk of text, possibly scrolling
type blurb interface {
	move(newStart Coord)                       //set line to draw at
	draw(force bool) (changed bool, err error) //draw on given line; possibly no-op if force is false
	fits() bool
	setTick(<-chan time.Time)
	debug(bool)
}

//Chooses scrolling type best suited for text size and allowed dims, returns
//blurb implementing this. Caution: legend width must not change once blurb is
//created, or blurb is likely to fail to draw.
func NewBlurb(l *Lcd, txt LcdTxt, start, dims Coord) blurb {
	lines := fit(txt, Coord{Col: dims.Col, Row: 255})
	if len(lines) <= int(dims.Row)+1 {
		return NewNoScroller(l, lines, start, dims)
	}
	if dims.Row > 0 {
		return NewVertScroller(l, lines, start, dims)
	}
	return NewHorizScroller(l, txt, start, dims)
}

//a blurb without scrolling
type noScroller struct {
	l                    *Lcd     //lcd this is displayed on
	txt                  []LcdTxt //text to display
	start, dims, lastPos Coord
	dbg                  bool
}

func NewNoScroller(l *Lcd, txt []LcdTxt, start, dims Coord) blurb {
	ns := &noScroller{
		l:       l,
		txt:     txt,
		start:   start,
		dims:    dims,
		lastPos: Coord{Row: offScreen},
	}
	return ns
}
func (ns *noScroller) move(newStart Coord) { ns.start = newStart }

func (ns *noScroller) draw(force bool) (bool, error) {
	if ns.dbg {
		log.Logf("ns draw")
	}
	if !force && ns.lastPos == ns.start {
		return false, nil
	}
	err := ns.l.writeWindow(ns.start, ns.dims, ns.txt)
	if err != nil {
		ns.lastPos.Row = offScreen //ensure we redraw next time
		return false, err
	}
	ns.lastPos = ns.start
	return true, nil
}

func (*noScroller) fits() bool               { return true }
func (*noScroller) setTick(<-chan time.Time) { /*throw it away*/ }
func (ns *noScroller) debug(b bool)          { ns.dbg = b }

var _ blurb = &noScroller{}

//a blurb with vertical scrolling - unlimited characters, constrained to a region on screen
type vertScroller struct {
	start, dims Coord
	lines       []LcdTxt
	topIdx      byte
	l           *Lcd
	tick        <-chan time.Time
	dbg         bool
}

func (vs *vertScroller) move(newStart Coord) {
	vs.topIdx = 0
	vs.start = newStart
}

func (vs *vertScroller) draw(force bool) (bool, error) {
	if vs.dbg {
		log.Logf("vs draw")
	}
	if vs.tick != nil {
		<-vs.tick
	}

	err := vs.l.writeWindow(vs.start, vs.dims, vs.lines[vs.topIdx:])
	if err != nil {
		return false, err
	}
	if int(vs.topIdx) >= len(vs.lines) {
		vs.topIdx = 0
	} else {
		vs.topIdx++
	}
	return true, nil
}

func (*vertScroller) fits() bool                    { return false }
func (vs *vertScroller) setTick(t <-chan time.Time) { vs.tick = t }
func (vs *vertScroller) debug(b bool)               { vs.dbg = b }

var _ blurb = &vertScroller{}

func NewVertScroller(l *Lcd, lines []LcdTxt, start, dims Coord) blurb {
	vs := &vertScroller{
		l:     l,
		start: start,
		dims:  dims,
		lines: lines,
	}
	return vs
}

//a blurb with horizontal scrolling - unlimited characters, but constrained to a single line
type horizScroller struct {
	l                    *Lcd   //lcd this is displayed on
	txt                  LcdTxt //text to display, automatically scrolls if too long
	availChars           byte   //number of chars available on display, taking into account overlay
	pos                  uint   //scroll position. stored so some redraws can be avoided
	scrollState          uint   //state for updateScrollPos(), related to pos but not identical
	start, dims, lastPos Coord  //note that 0 for row and/or col is valid, even in dims
	tick                 <-chan time.Time
	lastDrawn            LcdTxt
	dbg                  bool
}

var _ blurb = &horizScroller{}

func NewHorizScroller(l *Lcd, txt LcdTxt, start, dims Coord) blurb {
	hs := &horizScroller{
		l:           l,
		txt:         txt,
		availChars:  l.Width(),
		start:       start,
		dims:        dims,
		lastPos:     Coord{offScreen, offScreen},
		scrollState: 1, //first time through, this reduces the pre-scroll delay
	}
	return hs
}

//draw the item if it's on a different line than last time or if it
//needs to scroll horizontally
func (hs *horizScroller) draw(force bool) (changed bool, err error) {
	if hs.dbg {
		log.Logf("hs draw")
	}
	var rowChanged, scrollChanged bool
	if hs.lastPos != hs.start {
		//it is on a different line
		hs.lastPos = hs.start
		rowChanged = true
	}

	//act as if we got a tick if the channel is nil
	haveTick := true
	if hs.tick != nil {
		select {
		case <-hs.tick:
		default:
			haveTick = false
		}
	}
	//only scroll the item if it was not redrawn recently
	if haveTick {
		hs.updateState()
	}
	txt := visible(hs.txt, byte(hs.pos), hs.dims.Col, true)
	scrollChanged = !bytes.Equal(txt, hs.lastDrawn)
	if scrollChanged {
		hs.lastDrawn = txt
	}
	changed = force || rowChanged || scrollChanged
	if changed {
		err = hs.l.write(hs.start, txt, false)
	}
	return
}
func (*horizScroller) fits() bool { return false }
func (hs *horizScroller) move(newStart Coord) {
	hs.start = newStart
	if hs.start.Row == offScreen {
		//reset scroll state if we aren't visible
		hs.scrollState = 0
	}
}

//for ease of reading, doesn't start scrolling immediately or immediately rewind at end
func (hs *horizScroller) updateState() (change bool) {
	change = true
	hs.pos = hs.scrollState
	if hs.pos <= 2 {
		if hs.pos > 0 {
			//if 0, just updated from finalPos - so in that case, it did change. otherwise no
			change = false
		}
		hs.pos = 0
	} else {
		hs.pos -= 2
	}
	//calculate point at which no more scrolling is needed
	finalPos := uint(len(hs.txt)) - uint(hs.availChars)
	if hs.pos > finalPos {
		hs.pos = finalPos
		change = false
	}
	hs.scrollState++
	if hs.scrollState > finalPos+4 {
		hs.scrollState = 0
	}
	return
}
func (hs *horizScroller) setTick(t <-chan time.Time) { hs.tick = t }
func (hs *horizScroller) debug(b bool)               { hs.dbg = b }
