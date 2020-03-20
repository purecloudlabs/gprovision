// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

// +build !release

package cfa

import (
	"bytes"
	"fmt"
	"gprovision/pkg/log"
	"io"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"
)

var MockVerbose bool = true
var TraceEnter bool

func Mock(m Model, wg *sync.WaitGroup, useHex bool) (sd *SerialDev) {
	sd = &SerialDev{
		port: &mockPort{
			buf:       new(bytes.Buffer),
			responses: make(chan mockPkt, 5),
			model:     m,
			hex:       useHex,
		},
		In:             make(chan *Packet, 5),
		Events:         make(chan KeyActivity, 10),
		MinPktInterval: 10 * time.Millisecond,
		done:           make(chan struct{}),
	}
	if wg != nil {
		wg.Add(1)
	}
	go sd.handleIncoming(wg)
	return
}

type mockPort struct {
	closed     bool
	buf        *bytes.Buffer
	responses  chan mockPkt
	currentPkt mockPkt
	mtx        sync.Mutex
	model      Model
	hex        bool //if true, print reads/writes with hex instead of strings
}
type mockPkt []byte

func (m *mockPort) Close() (err error) {
	defer tracef("Close()")("=%s", &err)
	m.mtx.Lock()
	defer m.mtx.Unlock()
	if m.closed {
		return os.ErrClosed
	}
	close(m.responses)
	m.closed = true
	return
}

func (m *mockPort) Flush() (err error) {
	defer tracef("Flush()")("=%s", &err)
	m.mtx.Lock()
	defer m.mtx.Unlock()
	m.currentPkt = mockPkt{}
	for len(m.responses) > 0 {
		//discard
		<-m.responses
	}
	return nil
}

func (m *mockPort) Write(b []byte) (n int, err error) {
	tfmt := "Write(%q)"
	if m.hex {
		tfmt = "Write(%#v)"
	}
	defer tracef(tfmt, b)("=(%d,%s)", &n, &err)
	m.mtx.Lock()
	defer m.mtx.Unlock()
	if m.closed {
		return 0, os.ErrClosed
	}

	n = len(b)
	if n == 0 {
		log.Logf("0-length packet")
		return
	}
	//for simplicity, assume all Write() calls will be for exactly one packet
	if len(m.responses) == cap(m.responses) {
		log.Logf("discarding packet %v", b)
		n = 0
		err = io.ErrShortWrite
		return
	}
	m.translate(b)
	return
}

func (m *mockPort) translate(b []byte) {
	cmd := Command(b[0])
	dataLen := b[1]
	if int(dataLen) != len(b)-4 {
		log.Logf("data len and packet len mismatch in %v", b)
	}
	switch cmd {
	case Cmd_Clear:
		fallthrough
	case Cmd_CfgKeyReports:
		fallthrough
	case Cmd_SetBacklight:
		fallthrough
	case Cmd_SetCursorPos:
		fallthrough
	case Cmd_SetCursorStyle:
		fallthrough
	case Cmd_KeyLegendOnOffMask:
		fallthrough
	case Cmd_WriteDisp:
		m.minimalResponse(CFResponse, cmd)
	case Cmd_HwFwVers:
		ver := []byte("CFA631:hXvX,uYvY")
		if m.model == Cfa635 {
			ver = []byte("CFA635:hXvX,uYvY")
		}
		m.respond(CFResponse, cmd, ver)
	case Cmd_ReadReprtStat:
		rpt := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
		m.respond(CFResponse, cmd, rpt)
	case Cmd_Ping:
		pingData := b[2 : dataLen+2]
		m.respond(CFResponse, cmd, pingData)
	case Cmd_ReadKeysPolled:
		keys := []byte{0, 0, 0} //no key activity
		m.respond(CFResponse, cmd, keys)
	default:
		log.Logf("unknown command %s", cmd)
	}
}

//create response with no data
func (m *mockPort) minimalResponse(t PktType, c Command) {
	m.respond(t, c, nil)
}

//create response with data
func (m *mockPort) respond(t PktType, c Command, data []byte) {
	resp := &pktNoCrc{
		command:     (Command(t) << 6) | c,
		data_length: byte(len(data)),
	}
	copy(resp.data[:], data)
	buf, err := resp.buf(false)
	if err == nil {
		m.responses <- mockPkt(buf.Bytes())
	} else {
		log.Logf("translating mock packet: %s", err)
	}
}

func (m *mockPort) Read(b []byte) (n int, err error) {
	m.mtx.Lock()
	closed := m.closed
	m.mtx.Unlock()
	if closed {
		return 0, os.ErrClosed
	}

	rfmt := " [b=%q]  =(%d,%s)"
	if m.hex {
		rfmt = " [b=%#v]  =(%d,%s)"
	}
	defer tracef("Read(b)")(rfmt, b, &n, &err)

	if len(m.currentPkt) == 0 {
		m.currentPkt = <-m.responses
	}
	n = copy(b, m.currentPkt)
	m.currentPkt = m.currentPkt[n:]
	return
}

var _ SerialPort = &mockPort{}

//trace enter/exit with args. Pass pointers to capture return values (exception: slice).
//Assumes last arg to returned func is of type error.
//
//ex: defer tracef("Read(b)")(" [b=%q]  =(%d,%s)", b, &n, &err)
func tracef(f string, va ...interface{}) func(rfmt string, vb ...interface{}) {
	if !MockVerbose {
		return func(string, ...interface{}) {}
	}
	callStr := fmt.Sprintf(f, va...)
	retStr := callStr
	if TraceEnter {
		log.Logf(">  %s", callStr)
		retStr = " < " + callStr
	}
	return func(rfmt string, vb ...interface{}) {
		vn := len(vb)
		estr := "<nil>"
		for i, v := range vb {
			t := reflect.TypeOf(v)
			if t == nil {
				continue
			}
			switch t.Kind() {
			case reflect.Ptr:
				vb[i] = reflect.ValueOf(v).Elem().Interface()
			}
		}
		if vn > 0 {
			if vb[vn-1] != nil {
				estr = `"` + vb[vn-1].(error).Error() + `"`
			}
			vb[vn-1] = estr
		}
		log.Logf(retStr+rfmt, vb...)
	}
}

//decode a line like Write([]byte{0x20, 0x1, 0x0, 0x2f, 0xdc})=(5,<nil>)
//decodes writes only, reads are passed verbatim and others are suppressed.
func decodeOp(op string) *operation {
	op = strings.TrimPrefix(op, "LOG:")
	oparens := strings.Split(op, "(")
	if len(oparens) < 2 || len(oparens) > 3 {
		log.Logf("failed to parse %s, skipping", op)
		return nil
	}
	if oparens[0] == "Read" {
		//cannot prettyprint without a way of merging subsequent reads...
		return &operation{data: []byte(op)}
	}
	if oparens[0] != "Write" {
		log.Logf("skipping op %s in %s", oparens[0], op)
		return nil
	}
	bytestr := strings.TrimPrefix(oparens[1], "b) [b=")
	bytestr = strings.TrimPrefix(bytestr, "[]byte{")
	bytestr = strings.TrimSuffix(bytestr, "})=")
	bytestr = strings.TrimSuffix(bytestr, "}]  =")
	bytestrs := strings.Split(bytestr, ",")
	o := &operation{}
	var bytes []byte
	for i, b := range bytestrs {
		b = strings.TrimSpace(b)
		b = strings.TrimPrefix(b, "0x")
		n, err := strconv.ParseUint(b, 16, 8)
		if err != nil {
			log.Logf("failed to parse byte %d in %s, skipping: %s", i, op, err)
			return nil
		}
		if i == 0 {
			o.cmd = Command(n)
		} else if i == 1 {
			if int(n) != len(bytestrs)-4 {
				log.Logf("failed to parse %s - bad len. got %d want %d", op, n, len(bytestrs)-4)
				return nil
			}
		} else {
			bytes = append(bytes, byte(n))
		}
	}
	dlen := len(bytes) - 2
	o.write = oparens[0] == "Write"
	o.data = bytes[:dlen]
	o.crc = bytes[dlen:]

	return o
}

type operation struct {
	write     bool
	cmd       Command
	data, crc []byte
}

func (op *operation) String() string {
	if op == nil {
		return ""
	}
	if !op.write {
		//FIXME cannot pretty-print read packets now since each is 2-3 read ops
		return fmt.Sprintf(" <%s", string(op.data))
	}
	var s string
	s = "> Write " + op.cmd.String()
	switch op.cmd {
	case Cmd_WriteDisp:
		s += fmt.Sprintf(", @{c%02d,r%d} txt=%q", op.data[0], op.data[1], op.data[2:])
	case Cmd_Ping:
		s += fmt.Sprintf(", data={% 2x}", op.data)
	case Cmd_SetCursorStyle:
		s += fmt.Sprintf("=%d", op.data[0])
	case Cmd_SetCursorPos:
		fallthrough
	case Cmd_CfgKeyReports:
		fallthrough
	case Cmd_KeyLegendOnOffMask:
		s += fmt.Sprintf("={% 2x}", op.data)
	case Cmd_ReadReprtStat:
		//do nothing
	case Cmd_HwFwVers:
	case Cmd_Clear:
	default:
		if len(op.data) > 0 {
			s += fmt.Sprintf(", dlen=%d, data={% 2x}, crc={% 2x}", len(op.data), op.data, op.crc)
		} else {
			s += fmt.Sprintf(", dlen=%d, crc={% 2x}", len(op.data), op.crc)
		}
	}
	return s
}

func decode(in string) string {
	if strings.HasPrefix(in, "LOG:Write seq") {
		//a line injected into the log to delimit activity
		return in
	}
	return decodeOp(in).String()
}

//satisfied by gprovision/pkg/log/testlog
type Tlog interface {
	TstErrf(f string, va ...interface{})
	TstLogf(f string, va ...interface{})
	Freeze()
	Logf(f string, va ...interface{})
}

type mockKeySequence []struct {
	key    KeyActivity
	repeat int
}

type mockKeygen struct {
	l          *Lcd
	wg         sync.WaitGroup
	seq        mockKeySequence
	done       chan struct{}
	tlog       Tlog
	update     *Ticker
	timeout    time.Duration
	SyncTick   chan time.Time
	tickCount  int
	floodables []chan time.Time
	drainables []chan time.Time
}

func NewMockKeygen(l *Lcd, tlog Tlog, seq mockKeySequence, tickCount, headroom int) *mockKeygen {
	m := &mockKeygen{
		done:      make(chan struct{}),
		l:         l,
		tlog:      tlog,
		seq:       seq,
		SyncTick:  make(chan time.Time, headroom),
		tickCount: tickCount,
	}
	return m
}
func (m *mockKeygen) Floodable(c chan time.Time) { m.floodables = append(m.floodables, c) }
func (m *mockKeygen) Drainable(c chan time.Time) { m.drainables = append(m.drainables, c) }

func (m *mockKeygen) Run(timeout time.Duration) {
	m.timeout = timeout
	m.wg.Add(1)
	mt := NewMockTicker(m.tickCount, 5*time.Millisecond)
	m.update = NewTickerFromChan(make(chan time.Time)) //never signalled
	tick := mt.C
	go func() {
		defer mt.Stop()
		defer m.wg.Done()
		idx := 0
	outer:
		for {
			select {
			case <-tick:
				if idx < len(m.seq) {
					select {
					case m.l.dev.Events <- m.seq[idx].key:
					case <-time.After(m.timeout):
						m.tlog.TstErrf("event chan overflow at %d: %d", idx, m.seq[idx].key)
						continue outer
					}
					if m.seq[idx].repeat > 0 {
						m.seq[idx].repeat--
					} else {
						idx++
					}
					if idx == len(m.seq) {
						m.tlog.TstLogf("last key event sent")
					}
					<-m.SyncTick
				}
			case <-time.After(m.timeout):
				if idx < len(m.seq) {
					m.tlog.TstErrf("%d outstanding key events: idx=%d", len(m.seq)-idx, idx)
				}
				close(m.done)
				select {
				case m.l.dev.Events <- KEY_NO_KEY:
				case <-time.After(time.Millisecond):
				}
				time.Sleep(50 * time.Millisecond)
				m.tlog.Freeze()
				now := time.Now()
				// flood/drain channels
				for n := 0; n < 100; n++ {
					for _, t := range m.floodables {
						select {
						case t <- now:
						case <-time.After(time.Millisecond):
						}
					}
					for _, t := range m.drainables {
						select {
						case <-t:
						case <-time.After(time.Millisecond):
						}
					}
				}
				return
			}
		}
	}()
}
