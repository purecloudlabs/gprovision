// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package cfa

import (
	"bytes"
	"gprovision/pkg/log/testlog"
	"strings"
	"testing"
)

//func GetPacket(r ReadFlusher, DbgPktErr, DbgRW bool) (p *Packet, err error)
func TestGetPacket(t *testing.T) {
	tlog := testlog.NewTestLog(t, true, false)
	//crash reported by Aurelian, packet reproducing crash found through fuzzing
	pkt := []byte("I0000000000000000000000000000000000000000000000000000000")
	buf := bytes.NewBuffer(pkt)
	nf := &nopFlusher{r: buf}
	p, err := GetPacket(nf, true, true)
	if err == nil {
		t.Error("no error")
	}
	if p != nil {
		t.Errorf("got packet %s", p)
	}
	tlog.Freeze()
	if !strings.Contains(tlog.Buf.String(), "length out of range") {
		t.Errorf("expected out of range message to be logged")
	}
	if t.Failed() {
		t.Log(tlog.Buf.String())
	}
}
