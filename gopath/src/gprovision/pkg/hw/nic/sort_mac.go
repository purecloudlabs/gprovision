// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package nic

import (
	"bytes"
	"encoding/binary"
	"gprovision/pkg/common/strs"
	"net"
	"sort"
)

var be = binary.BigEndian

// Sort interfaces by mac, ascending.
// Filters out interfaces lacking an allowed prefix, if such are specified.
func SortedList(allowedPrefixes [][]byte) NicList {
	all := List()
	filtered := all.FilterMACs(allowedPrefixes)
	return filtered.Sort()
}

//Return Nics whose MACs match allowedPrefixes. Return all Nics if list is empty.
func (nl NicList) FilterMACs(allowedPrefixes [][]byte) (filtered NicList) {
	for _, n := range nl {
		if len(allowedPrefixes) == 0 || n.AllowedPrefix(allowedPrefixes) {
			filtered = append(filtered, n)
		}
	}
	return
}

type FilterFn func(int, Nic) bool

func (nl NicList) Filter(fn FilterFn) (filtered NicList) {
	for i, n := range nl {
		if fn(i, n) {
			filtered = append(filtered, n)
		}
	}
	return
}

//A filter for nics with the given indexes.
func IndexFilter(indexes []int) FilterFn {
	return func(idx int, n Nic) bool {
		for _, i := range indexes {
			if idx == i {
				return true
			}
		}
		return false
	}
}

//A filter that inverts the sense of the given filter.
func NotFilter(fn FilterFn) FilterFn {
	return func(idx int, n Nic) bool {
		return !fn(idx, n)
	}
}

func (nl NicList) Sort() (sorted NicList) {
	var macs []net.HardwareAddr
	for _, n := range nl {
		macs = append(macs, n.mac)
	}
	smacs := SortableMacs(macs)
	smacs.Sort()
	for _, m := range smacs {
		for _, n := range nl {
			if bytes.Equal(n.mac, m.Mac()) {
				sorted = append(sorted, n)
				break
			}
		}
	}
	return
}

func (n Nic) MatchOUI() bool {
	pfx := strs.MacOUIBytes()
	return bytes.Equal(pfx, n.mac[:len(pfx)])
}
func (n Nic) AllowedPrefix(allowed [][]byte) bool {
	for _, pfx := range allowed {
		if bytes.Equal(pfx, n.mac[:len(pfx)]) {
			return true
		}
	}
	return false
}

//macs are converted from []byte to uint64 to make it easier to do the sort comparisons
type umac uint64
type MACList []umac

func SortableMacs(l []net.HardwareAddr) (s MACList) {
	s = make(MACList, len(l))
	for idx, a := range l {
		//Uint64/PutUint64 require 8 bytes; MACs are 6 bytes so an offset is required when copied/compared
		var b [8]byte
		copy(b[2:], a)
		i := be.Uint64(b[:])
		s[idx] = umac(i)
	}
	return
}
func (m MACList) Sort() { sort.Sort(m) }

func (m MACList) Len() int           { return len(m) }
func (m MACList) Swap(i, j int)      { m[i], m[j] = m[j], m[i] }
func (m MACList) Less(i, j int) bool { return m[i] < m[j] }

func (m MACList) Sequential() bool {
	maxidx := len(m) - 1
	for i := 0; i < maxidx; i++ {
		if m[i]+1 != m[i+1] {
			return false
		}
	}
	return true
}

func (m umac) Mac() net.HardwareAddr {
	var b [8]byte
	be.PutUint64(b[:], uint64(m))
	return b[2:]
}
