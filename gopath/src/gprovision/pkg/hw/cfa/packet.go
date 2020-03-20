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
	"gprovision/pkg/log"
	"io"
)

const MAX_DATA_LENGTH = 22

//used for outgoing packets as we don't calculate the crc until the last moment
type pktNoCrc struct {
	command     Command
	data_length byte
	data        [MAX_DATA_LENGTH]byte
}

//incoming packets do include the crc
type Packet struct {
	pktNoCrc
	crc uint16
}

func (p *pktNoCrc) SetCommand(cmd Command) error {
	if cmd&0xC0 != 0 {
		return fmt.Errorf("command out of range: %x", cmd)
	}
	p.command = cmd
	return nil
}

func (p *pktNoCrc) SetData(data []byte) error {
	l := len(data)
	if l > MAX_DATA_LENGTH {
		return fmt.Errorf("data oversized: %d %x", len(data), data)
	}
	p.data_length = byte(l)
	copy(p.data[:], data)
	return nil
}

type PktType byte

const (
	CFCommand  PktType = iota //0 (00)
	CFResponse                //1 (01)
	CFReport                  //2 (10)
	CFError                   //3 (11)
)

func (p *pktNoCrc) Type() PktType {
	return PktType(p.command >> 6)
}

//note that the returned value includes the `type` bits
func (p *pktNoCrc) Cmd() Command {
	return p.command
}

func (p *pktNoCrc) Data() []byte {
	return p.data[:p.data_length]
}

func CfCrc(bytes []byte) (crc uint16) {
	crc = 0xffff
	for _, b := range bytes {
		for i := 8; i > 0; i-- {
			if (crc^uint16(b))&0x01 == 1 {
				crc >>= 1
				crc ^= 0x8408
			} else {
				crc >>= 1
			}
			b >>= 1
		}
	}
	crc = ^crc

	//swap bytes
	crc = ((crc << 8) & 0xff00) | (crc >> 8)
	return
}

//create 26-byte buffer
func pktbuf() *bytes.Buffer {
	/* bufSize is a slice of length 0, capacity 26 -
	   sets size of buffer with a minimum of alloc's */
	bufSize := make([]byte, 0, 26)
	return bytes.NewBuffer(bufSize)
}

//Calculate Crystalfontz CRC for packet
func (p *pktNoCrc) Crc() (crc uint16, buf *bytes.Buffer, err error) {
	if len(p.data) < int(p.data_length) {
		err = io.ErrShortBuffer
		return
	}
	buf = pktbuf()
	err = buf.WriteByte(byte(p.command))
	if err == nil {
		err = buf.WriteByte(p.data_length)
	}
	if err == nil {
		_, err = buf.Write(p.data[:p.data_length])
	}
	if err == nil {
		crc = CfCrc(buf.Bytes())
		return
	}
	buf = nil
	return
}

//calculate crc, write packet out
func (p *pktNoCrc) WriteTo(w io.Writer, verbose bool) error {
	buf, err := p.buf(verbose)
	if err == nil {
		_, err = buf.WriteTo(w)
	}
	return err

}
func (p *pktNoCrc) buf(verbose bool) (*bytes.Buffer, error) {
	crc, buf, err := p.Crc()
	if err == nil {
		err = binary.Write(buf, binary.BigEndian, crc)
	}
	if err != nil {
		buf = nil
	} else if verbose {
		p.Log(crc, false)
	}
	return buf, err
}

//check crc
func (p *Packet) Validate() bool {
	crc, _, err := p.pktNoCrc.Crc()
	if err != nil {
		return false
	}
	return crc == p.crc
}

//Logs packet (+CRC) and whether it was read or written.
func (p *pktNoCrc) Log(crc uint16, read bool) {
	format := "%s %-70s (crc %04x)"
	dir := " ->"
	if read {
		dir = "<- "
	}
	log.Logf(format, dir, p, crc)
}

//String method so that Printf (etc) can easily render the packet
func (p *pktNoCrc) String() string {
	if p.data_length == 0 {
		return fmt.Sprintf("%-28s <no data>", p.command)
	}
	return fmt.Sprintf("%-28s [%02d]data=%q", p.command, p.data_length, p.data[:p.data_length])
}
