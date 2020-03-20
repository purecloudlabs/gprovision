// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package cfa

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"gprovision/pkg/hw/cfa/serial"
	"gprovision/pkg/log"
	"io"
	"os"
	"sync"
	"time"
)

//consts related to packet transmission
const (
	MaxTries           = 5                      //number of re-transmissions before giving up
	PktResponseWaitMax = 300 * time.Millisecond //pdf says 250mS + OS overhead
)

var ERetry = fmt.Errorf("Reached max number of retries")
var EPacket = fmt.Errorf("Bad packet")

//SerialDev translates Crystalfontz-compatible packets into serial byte streams
// and vice versa. It intercepts and decodes incoming key reporting and polling
// packets, populating Events. Other packets go into In.
type SerialDev struct {
	port   SerialPort
	done   chan struct{}
	Events chan KeyActivity //incoming button/key events
	In     chan *Packet     //incoming packets except above events

	pktTimeMtx sync.Mutex //guard for lastPkt
	lastPkt    time.Time  //time at which last packet was sent/received

	// xlateTable is used to translate polled key data to events. Init is a bit
	// chicken-and-egg since we need the model to init this, and the serial dev
	// must be up to discover the model.
	XlateTable []KeyXlate

	DbgRW          bool          //if true, log content of every packet read or written
	DbgPktErr      bool          //if true, log packet errors even if they will be retried
	MinPktInterval time.Duration //dead time between packets
}

type SerialPort interface {
	Close() error
	Read([]byte) (int, error)
	Write([]byte) (int, error)
	Flush() error
}

//set up port, init channels, start bg process
func SerialSetup(dev string) (sd *SerialDev, err error) {
	sd = &SerialDev{MinPktInterval: 10 * time.Millisecond}
	sd.port, err = serial.Open(dev)
	if err != nil {
		sd = nil
		return
	}
	//channel sizes are arbitrary. We _probably_ never have more than a single
	//entry at a time, but the costs are negligible for a bit of extra storage
	sd.In = make(chan *Packet, 5)
	sd.Events = make(chan KeyActivity, 10)

	sd.done = make(chan struct{})
	go sd.handleIncoming(nil)
	return
}

/*
Stuffs incoming events into event channel, and other packets into the packet
channel. Must run in background.
*/
func (sd *SerialDev) handleIncoming(wg *sync.WaitGroup) {
	if wg != nil {
		defer wg.Done()
	}
	defer close(sd.In)
	defer close(sd.Events)
	var lastErr error
	for {
		select {
		case <-sd.done:
			return
		default:
		}
		p, err := GetPacket(sd.port, sd.DbgPktErr, sd.DbgRW)
		if err == os.ErrClosed {
			return
		}
		if err != nil {
			//only print an error if it's different than previous
			if lastErr != err {
				if err == io.ErrUnexpectedEOF {
					log.Logf("lost connection to lcd")
				} else if sd.DbgPktErr {
					log.Logf("error reading from lcd: %s", err)
				}
				lastErr = err
			}
			time.Sleep(20 * time.Millisecond)
			continue
		}
		lastErr = nil
		if p == nil {
			//should never get here, right?
			log.Logf("handleIncoming: p is nil")
			continue
		}
		sd.setPktTime()
		if p.command == Report_Key && p.data_length == 1 {
			evt := p.data[0]
			if len(sd.Events) == cap(sd.Events) {
				//full buffer
				if sd.DbgRW {
					log.Logf("discarding event %x", evt)
				}
				continue
			}
			sd.Events <- KeyActivity(evt)
		} else if p.command.CommandFromResponse() == Cmd_ReadKeysPolled {
			if p.data_length != 3 {
				if sd.DbgPktErr {
					log.Logf("bad length for %s: [%d]data=%q", p.command, p.data_length, p.data)
				}
				continue
			}
			if len(sd.XlateTable) == 0 {
				if sd.DbgPktErr {
					log.Logf("WARNING key poll packet received, but translation table not set up. %#v", sd.XlateTable)
				}
				continue
			}
			mask := KeyMask(p.data[KeyPollReleased])
			for _, ka := range keymaskToActivity(sd.XlateTable, mask) {
				sd.Events <- ka
			}
		} else {
			if len(sd.In) == cap(sd.In) {
				//full buffer
				if sd.DbgRW {
					log.Logf("discarding packet %s", p)
				}
				continue
			}
			sd.In <- p
		}
	}
}

func (sd *SerialDev) Close() error {
	close(sd.done)
	return sd.port.Close()
}

//Send a packet, retrying up to MaxTries
func (sd *SerialDev) sendPktRetry(pkt *pktNoCrc) (*Packet, error) {
	//getPkt introduces a delay when there is no response so do not add additional delay here
	for r := MaxTries; r >= 0; r-- {
		p, err := sd.sendPkt(pkt)
		if p != nil || err != nil {
			return p, err
		}
		success, _ := sd.ping()
		if sd.DbgPktErr {
			if success {
				log.Logf("lost packet but ping was successful")
			} else {
				log.Logf("lost packet, no ping response")
			}
		}
	}
	return nil, ERetry
}

//send packet, wait for response. no retries.
func (sd *SerialDev) sendPkt(pkt *pktNoCrc) (p *Packet, err error) {
	err = sd.sendOnly(pkt)
	if err == nil {
		p = sd.getPkt(pkt.command, PktResponseWaitMax)
	}
	return
}

//send packet, do not wait for response
func (sd *SerialDev) sendOnly(pkt *pktNoCrc) (err error) {
	now := time.Now()
	last := sd.getPktTime()
	if last.Add(sd.MinPktInterval).After(now) {
		time.Sleep(last.Add(sd.MinPktInterval).Sub(now))
	}
	sd.setPktTime()
	err = pkt.WriteTo(sd.port, sd.DbgRW)
	return
}

//get matching incoming packet with timeout
func (sd *SerialDev) getPkt(cmd Command, maxWait time.Duration) (p *Packet) {
	for {
		select {
		case p = <-sd.In:
			sd.setPktTime()
			if p.command.CommandFromResponse() == cmd {
				return
			} else if sd.DbgPktErr {
				log.Logf("got unexpected packet %s", p)
			}
		case <-time.After(maxWait):
			if sd.DbgPktErr {
				log.Logf("no response to %s pkt", cmd)
			}
			p = nil
			return
		}
	}
}
func (sd *SerialDev) setPktTime() {
	sd.pktTimeMtx.Lock()
	defer sd.pktTimeMtx.Unlock()
	sd.lastPkt = time.Now()
}
func (sd *SerialDev) getPktTime() time.Time {
	sd.pktTimeMtx.Lock()
	defer sd.pktTimeMtx.Unlock()
	return sd.lastPkt
}

//sends ping command with random-ish data, expects it back
func (sd *SerialDev) ping() (match bool, err error) {
	p := &pktNoCrc{command: Cmd_Ping}

	//get random(ish) value
	nano := uint64(time.Now().UnixNano())
	binary.LittleEndian.PutUint64(p.data[:], nano)
	p.data_length = 8
	var resp *Packet
	resp, err = sd.sendPkt(p)
	if err == nil && resp != nil {
		match = (resp.data == p.data)
	}
	return match, err
}

//type passed to Read() and badPacket(); subset of SerialPort
type ReadFlusher interface {
	Read([]byte) (int, error)
	Flush() error
}

//Stuffs incoming data from the serial port into packets. Checks packet type, verifies CRC.
func GetPacket(r ReadFlusher, DbgPktErr, DbgRW bool) (p *Packet, err error) {
	//NOTE: do not do buf.ReadFrom(r) - it will block. interpose an io.LimitReader.
	buf := pktbuf()
	var n int64
	n, err = buf.ReadFrom(io.LimitReader(r, 2))
	if err == nil && n != 2 {
		if n != 0 {
			if DbgPktErr {
				log.Logf("Read() wants 2, got %d: 0x%x", n, buf.Bytes())
			}
		}
		err = io.ErrUnexpectedEOF
	}
	if err != nil {
		return
	}
	cmd := Command(buf.Bytes()[0])
	switch cmd.Type() {
	case CFReport:
		//keypress reports are handled with CFResponse, below
		if cmd != Report_Key {
			return badPacket(cmd, buf, r, "received unhandled report packet", DbgPktErr)
		}
	case CFResponse:
		//most common. handle below
	case CFError:
		return badPacket(cmd, buf, r, "received error packet", DbgPktErr)
	default:
		return badPacket(cmd, buf, r, "received unknown packet", DbgPktErr)
	}
	p = &Packet{}
	p.command = cmd
	p.data_length = buf.Bytes()[1]
	if p.data_length > MAX_DATA_LENGTH {
		return badPacket(cmd, buf, r, "data_length out of range", DbgPktErr)
	}
	//read data, if present
	if p.data_length != 0 {
		n, err = buf.ReadFrom(io.LimitReader(r, int64(p.data_length)))
		if err == nil && n != int64(p.data_length) {
			err = io.ErrUnexpectedEOF
		}
		if err == nil {
			copied := copy(p.data[:], buf.Bytes()[2:p.data_length+2])
			if copied != int(p.data_length) {
				err = io.ErrUnexpectedEOF
			}
		}
		if err != nil {
			if DbgPktErr {
				log.Logf("error %s, n=%d, l=%d, read % x, data", err, n, p.data_length, buf.Bytes())
			}
			return
		}
	}
	//read CRC
	n, err = buf.ReadFrom(io.LimitReader(r, 2))
	if err == nil && n != 2 {
		err = io.ErrUnexpectedEOF
	}
	if err != nil {
		if DbgPktErr {
			log.Logf("error %s, n=%d, read % x, crc", err, n, buf.Bytes())
		}
		p.Log(0, true)
		return
	}

	//verify CRC
	if buf.Len() >= int(p.data_length+4) {
		crcbytes := buf.Bytes()[p.data_length+2 : p.data_length+4]
		p.crc = binary.BigEndian.Uint16(crcbytes)
		valid := p.Validate()
		if !valid {
			if DbgPktErr {
				log.Logf("invalid crc:")
				p.Log(p.crc, true)
			}
			return nil, os.ErrInvalid
		}
	} else {
		return badPacket(cmd, buf, r, "received undersized packet", DbgPktErr)
	}
	if DbgRW {
		p.Log(p.crc, true)
	}
	return
}

//log bad packet, flush r
func badPacket(cmd Command, buf *bytes.Buffer, r ReadFlusher, desc string, DbgRW bool) (*Packet, error) {
	if DbgRW {
		log.Logf("%s with cmd %s, buf=%v", desc, cmd, buf.Bytes())
	}
	r.Flush()
	return nil, EPacket
}
