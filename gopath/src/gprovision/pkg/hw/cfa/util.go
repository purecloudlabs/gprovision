// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package cfa

import (
	"bytes"
	"fmt"
	"gprovision/pkg/hw/cfa/serial"
	"gprovision/pkg/log"
	"io"
	"os"
	"strconv"
	"time"
)

//int minimum function
func imin(a, b int) int {
	if a < b {
		return a
	}
	return b
}

//break msg into chunks that fit within rect.
//limited to 256 lines... not that we're likely to hit that.
func fit(msg LcdTxt, rect Coord) []LcdTxt {
	var lines []LcdTxt
	for len(msg) > 0 {
		nextWrap := wrapPos(msg, rect.Col)
		if nextWrap == 0 {
			nextWrap = byte(imin(len(msg), int(rect.Col)))
		}
		lines = append(lines, msg[:nextWrap])
		msg = bytes.TrimLeft(msg[nextWrap:], " ")
		if len(lines) > int(rect.Row) {
			break
		}
	}
	return lines
}

//find a space at or before 'width'
func wrapPos(msg LcdTxt, width byte) byte {
	if len(msg) <= int(width) {
		return byte(len(msg))
	}
	i := bytes.LastIndex(msg[:width+1], []byte(" "))
	if i < 0 {
		i = 0
	}
	return byte(i)
}

//pad line to given size by appending spaces
func pad(line LcdTxt, w byte) []byte {
	spaces := int(w) - len(line)
	if spaces < 0 {
		log.Logf("pad: len('%s') > %d", line, w)
		spaces = 0
	}
	return []byte(fmt.Sprintf("%s%*s", line, spaces, ""))
}

//Write one or more lines to the display. Clears any existing text on all lines.
func (l *Lcd) writeLines(lines []LcdTxt, max Coord) (Coord, error) {
	var end Coord
	for line := byte(0); line <= max.Row; line++ {
		if int(line) < len(lines) {
			err := l.write(Coord{Row: line}, lines[line], true)
			if err != nil {
				return Coord{}, err
			}
			end.Row = line
			end.Col = byte(len(lines[line]))
		} else {
			//blank line
			err := l.write(Coord{Row: line}, LcdTxt{}, true)
			if err != nil {
				return Coord{}, err
			}
		}
	}
	return end, nil
}

//Writes text in rectangle defined by 'start' and 'dims'.
//Affects nothing outside that rectangle, but clears all text within.
func (l *Lcd) writeWindow(start, dims Coord, lines []LcdTxt) error {
	var r byte
	var idx int
	for r = start.Row; r <= start.Row+dims.Row; r++ {
		var line LcdTxt
		if idx < len(lines) {
			line = lines[idx]
		}
		sized := padcate(line, dims.Col)
		pos := Coord{Row: r, Col: start.Col}
		err := l.write(pos, sized, false)
		if err != nil {
			log.Logf("writing %q @ %v, window %v %v: %s", sized, pos, start, dims, err)
			return err
		}
		idx++
	}
	return nil
}

type dbgFlags int

const (
	DbgPktRW dbgFlags = 1 << iota
	DbgPktErr
	DbgGeneral
	DbgMenu
	DbgTraceSer
	DbgTraceSerEnter
)

func (l *Lcd) Debug(flags dbgFlags) {
	l.dev.DbgRW = (flags & DbgPktRW) != 0
	l.dev.DbgPktErr = (flags & DbgPktErr) != 0
	l.DbgGeneral = (flags & DbgGeneral) != 0
	l.DbgMenu = (flags & DbgMenu) != 0

	serExit := (flags & DbgTraceSer) != 0
	serEnter := (flags & DbgTraceSerEnter) != 0
	serial.Debug(serExit, serEnter)
}

func FlagsFromString(bits string) dbgFlags {
	i, err := strconv.ParseInt(bits, 2, 64)
	if err != nil {
		log.Logf("cannot parse %s as bits: %s", bits, err)
		fmt.Fprintf(os.Stderr, `Help for debug flags

This is bits represented as a string - i.e. 001000
Where rightmost is least significant and leftmost is most significant.
Position   Flag
0          DbgRW            - print out all packets as they are read or written to device
1          DbgPktErr        - print packet errors related to retries
2          DbgGeneral       - general debug info not covered elsewhere
3          DbgMenu          - debug info related to menus
4          DbgTraceSer      - trace serial pkg Read()/Write() on exit. Uses stderr; not in release builds.
5          DbgTraceSerEnter - as above but traces entrance. Requires DbgTraceSer.

Examples
001000  - DbgMenu=true; others false
000010  - DbgPktErr=true
000101  - DbgGeneral + DbgRW
`)
		os.Exit(1)
	}
	return dbgFlags(i)
}

//nopFlusher: used for fuzzing and testing. must implement ReadFlusher.
type nopFlusher struct {
	r io.Reader
}

var _ ReadFlusher = &nopFlusher{}

func (n *nopFlusher) Flush() error               { return nil }
func (n *nopFlusher) Read(b []byte) (int, error) { return n.r.Read(b) }

//Convert string arg(s) into array of LcdTxt. More compact/less painful than wrapping every
//individual string in LcdTxt()
func Strs2LTxt(strs ...string) []LcdTxt {
	var lt []LcdTxt
	for _, s := range strs {
		lt = append(lt, LcdTxt(s))
	}
	return lt
}

//return a slice of txt no longer than 'max'
func truncate(txt []byte, max byte) []byte {
	if len(txt) <= int(max) {
		return txt
	}
	return txt[:max]
}

//pad/truncate txt as necessary to attain width w
func padcate(txt LcdTxt, w byte) LcdTxt {
	return pad(truncate(txt, w), w)
}

//Function visible returns a section of LcdTxt that will be visible; if
//ellipsis is true, shows symbol(s) if some text is offscreen. Note that the
//symbols used are not actually ellipsis as those aren't present in the lcd
//character set. Symbols used are like << and >>.
func visible(txt LcdTxt, start, width byte, ellipsis bool) LcdTxt {
	l := len(txt)
	if int(start) >= l || width == 0 {
		return LcdTxt{}
	}
	vis := txt[start:]
	if len(vis) > int(width) {
		vis = vis[:width]
	}
	if !ellipsis || width <= 3 {
		return vis
	}
	output := make(LcdTxt, len(vis))
	copy(output[:], vis)
	if start != 0 {
		output[0] = SymLtLt // <<
	}
	if int(start+width) < len(txt) {
		output[len(output)-1] = SymGtGt // >>
	}
	return output
}

//can encompass a time.Ticker, or be a mock suitable for testing.
type Ticker struct {
	C    <-chan time.Time
	t    *time.Ticker
	In   chan time.Time
	done chan struct{}
}

//NewTicker creates a Ticker with behavior identical to time.Ticker.
func NewTicker(p time.Duration) *Ticker {
	t := &Ticker{
		t: time.NewTicker(p),
	}
	t.C = t.t.C
	return t
}

//NewMockTicker creates a ticker that will provide the given number of ticks
//and minimum interval between them.
func NewMockTicker(maxCount int, minInterval time.Duration) *Ticker {
	t := &Ticker{
		In:   make(chan time.Time),
		done: make(chan struct{}),
	}
	t.C = t.In
	go func() {
		for i := 0; i < maxCount; i++ {
			select {
			case <-t.done:
				break
			default:
			}
			n := time.Now()
			t.In <- n
			time.Sleep(minInterval)
		}
	}()
	return t
}

//NewTickerFromChan creates a Ticker whose C is the given channel. Use case:
//testing functions that take a ticker, but you have a TickDistrib instead.
func NewTickerFromChan(c <-chan time.Time) *Ticker {
	return &Ticker{
		C:    c,
		done: make(chan struct{}),
	}
}

func (t *Ticker) Stop() {
	if t.t != nil {
		t.t.Stop()
	} else {
		close(t.done)
		for len(t.C) > 0 {
			<-t.C //empty the channel without being blocked
		}
	}
}

//A TickDistrib copies each tick on 'in' to all output channels.
//Used to signal to multiple ui elements that they can update.
type TickDistrib struct {
	in     <-chan time.Time
	out    []chan time.Time
	done   chan struct{}
	debug  bool
	relent time.Duration
}

//create a TickDistrib with n output channels
func NewTickDistrib(in <-chan time.Time, n int) *TickDistrib {
	td := &TickDistrib{
		in:     in,
		done:   make(chan struct{}),
		relent: time.Millisecond * 3,
	}
	for i := 0; i < n; i++ {
		td.out = append(td.out, make(chan time.Time, 1))
	}
	go td.run()
	return td
}

//like NewTickDistrib, but for testing. exposes extra knobs
func NewTestTickDistrib(in <-chan time.Time, n int, relent time.Duration, depth int, dbg bool) *TickDistrib {
	td := &TickDistrib{
		in:     in,
		done:   make(chan struct{}),
		relent: relent,
		debug:  dbg,
	}
	for i := 0; i < n; i++ {
		td.out = append(td.out, make(chan time.Time, depth))
	}
	go td.run()
	return td
}

func (td *TickDistrib) Stop() { close(td.done) }
func (td *TickDistrib) run() {
	var t time.Time
	for {
		select {
		case <-td.done:
			for _, c := range td.out {
				close(c)
			}
			return
		case t = <-td.in:
			for i, c := range td.out {
				if td.relent > 0 {
					select {
					case c <- t:
					case <-time.After(td.relent):
						if td.debug {
							log.Logf("skipping tick on %d", i)
						}
					}
				} else {
					c <- t
				}
			}
		}
	}
}
func (td *TickDistrib) Get(i uint) <-chan time.Time {
	if int(i) >= len(td.out) {
		return nil
	}
	return td.out[i]
}
