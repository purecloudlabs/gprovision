// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package cfa

import (
	"testing"
)

//func CfCrc(bytes []byte) (crc uint16)
func TestCfCrc(t *testing.T) {
	//for your own sanity, do not attempt to compare these test vectors with online resources.
	//what matters is that this impl's output matches that of the crystalfontz impl, and it does.
	testdata := []struct {
		data string
		crc  uint16
	}{
		//first 4 match ITU X.25 Appendix I
		{"\x03\x3f", 0x5bec},
		{"\x01\x73", 0x8357},
		{"\x01\x3f", 0xebdf},
		{"\x03\x73", 0x3364},
		//nobody can agree on this one
		{"123456789", 0x6e90},
		//From Crystalfontz pdf... only the value given in the pdf disagrees with their impl's output
		{"This is a test. ", 0x78e4},
	}
	for i, j := range testdata {
		out := CfCrc([]byte(j.data))
		if out != j.crc {
			t.Errorf("mismatch at %d: want %x, got %x", i, j.crc, out)
		}
	}
}

func TestPacketCRC(t *testing.T) {
	p := &Packet{
		pktNoCrc: pktNoCrc{
			command:     0x0a,
			data_length: 5,
		},
		crc: 0x72a7,
	}
	copy(p.data[:], []byte("abcde"))
	crc, _, err := p.Crc()
	if err != nil {
		t.Error(err)
	}
	valid := p.Validate()
	if !valid {
		t.Error("invalid")
	}
	if t.Failed() {
		t.Logf("%x %t", crc, valid)
	}
}

func TestErrorPacket(t *testing.T) {
	l, err := Find()
	if err != nil {
		t.Skip(err)
	}
	/* 0x34: invalid command. tried 0x24 but that seems to be an
	undocumented command that causes a delay.
	*/
	p := &pktNoCrc{command: 0x34}
	pkt, err := l.dev.sendPkt(p)
	if pkt != nil {
		t.Error("expected no response")
	}
	if err != nil {
		t.Error(err)
	}
	err = l.write(Coord{0, 0}, []byte("after invalid cmd"), false)
	if err != nil {
		t.Error(err)
	}
}
