// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package nic

import (
	"bytes"
	"net"
	"testing"
)

func TestSortMacs1(t *testing.T) {
	nics, err := net.Interfaces()
	if err != nil {
		t.Errorf("getting nics: %s", err)
	}
	var macs []net.HardwareAddr
	for _, i := range nics {
		if len(i.HardwareAddr) == 0 {
			continue
		}
		macs = append(macs, i.HardwareAddr)
	}
	smacs := SortableMacs(macs)
	t.Logf("macs:\n%v\nsmacs:\n%v\n", macs, smacs)
	smacs.Sort()
	t.Logf("sorted smacs:\n%v\n", smacs)
	t.Logf("sequential: %t\n", smacs.Sequential())
}

type macTestData struct {
	macstrings []string
	sequential bool
}

func TestSortMacs2(t *testing.T) {
	data := []macTestData{
		{[]string{"00:26:FD:A0:34:A1", "00:26:FD:A0:34:A0"}, true},
		{[]string{"0c:c4:7a:6b:96:cc", "0c:c4:7a:6b:96:cd", "0c:c4:7a:6b:96:ce", "0c:c4:7a:6b:96:cf"}, true},
		{[]string{"00:26:FD:A0:34:A1", "00:26:FD:A0:34:A0", "00:26:FD:A0:34:A3"}, false},
		{[]string{"00:26:FD:A0:34:A1", "00:26:FD:A0:34:A0", "00:26:FD:A0:34:A0"}, false},
		{[]string{"00:26:FD:A0:34:A1"}, true},
	}
	for i, d := range data {
		var macs []net.HardwareAddr
		for _, a := range d.macstrings {
			m, err := net.ParseMAC(a)
			if err != nil {
				t.Errorf("can't parse mac %s: %s", a, err)
				continue
			}
			macs = append(macs, m)
		}
		smacs := SortableMacs(macs)
		smacs.Sort()
		if smacs.Sequential() != d.sequential {
			t.Errorf("test %d, sequentiality: got %t, want %t\n%v\n", i, smacs.Sequential(), d.sequential, smacs)
		}

	}
}

func TestSortMacs3(t *testing.T) {
	data := []struct {
		macstrings []string
		order      []int
	}{
		{[]string{"0c:c4:7a:6b:96:cc", "0c:c4:7a:6b:96:cd", "00:26:FD:A0:34:A1", "0c:c4:7a:6a:96:cd", "0d:c4:7a:6b:96:cd",
			"00:26:FD:A0:34:A0", "0c:c4:7a:6b:96:ce", "0c:c4:7a:6b:96:cf"}, []int{5, 2, 3, 0, 1, 6, 7, 4}},
	}
	for i, d := range data {
		if len(d.macstrings) != len(d.order) {
			t.Errorf("bad test data %d %#v", i, d)
		}
		var macs []net.HardwareAddr
		for _, a := range d.macstrings {
			m, err := net.ParseMAC(a)
			if err != nil {
				t.Errorf("can't parse mac %s: %s", a, err)
				continue
			}
			macs = append(macs, m)
		}
		smacs := SortableMacs(macs)
		smacs.Sort()
		for j, k := range d.order {
			if !bytes.Equal(smacs[j].Mac(), macs[k]) {
				t.Errorf("row %d (%d,%d): want %s, got %s", i, j, k, macs[k], smacs[j].Mac())
			}
		}
	}
}

//tests that a sorted list remains sorted and that the individual macs come out unchanged
func TestSortedMacs(t *testing.T) {
	data := []string{"0c:c4:7a:6b:96:cc", "0c:c4:7a:6b:96:cd", "0c:c4:7a:6b:96:ce", "0c:c4:7a:6b:96:cf"}
	var macs []net.HardwareAddr
	for _, a := range data {
		m, err := net.ParseMAC(a)
		if err != nil {
			t.Error(err)
			continue
		}
		macs = append(macs, m)
	}
	smacs := SortableMacs(macs)
	smacs.Sort()
	for i, m := range smacs {
		s := m.Mac().String()
		if s != data[i] {
			t.Errorf("got %s, want %s", s, data[i])
		}
	}
}
