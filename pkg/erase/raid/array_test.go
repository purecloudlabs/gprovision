// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package raid

import (
	"testing"

	"github.com/purecloudlabs/gprovision/pkg/log/testlog"
)

//func FindArrays(devices []*Device) (arrays []*Array)
func TestFindArrays(t *testing.T) {
	singleIn := Devices{&Device{devSize: 1234, arrayType: unknown}}
	singleWant := Arrays{&Array{devices: singleIn}}
	multiUnknownIn := Devices{
		&Device{devSize: 5678, arrayType: unknown},
		&Device{devSize: 5679, arrayType: unknown},
	}
	multiUnknownWant := Arrays{&Array{devices: multiUnknownIn}}
	multiUnknownIn2 := append(multiUnknownIn, singleIn[0])
	multiUnknownWant2 := append(multiUnknownWant, &Array{devices: singleIn})
	testdata := []struct {
		name string
		in   Devices
		want Arrays
		fail bool
	}{
		{
			name: "single",
			in:   singleIn,
			want: singleWant,
		},
		{
			name: "multiUnknown",
			in:   multiUnknownIn,
			want: multiUnknownWant,
		},
		{
			name: "multiUnknown2",
			in:   multiUnknownIn2,
			want: multiUnknownWant2,
		},
	}
	for _, td := range testdata {
		t.Run(td.name, func(t *testing.T) {
			tlog := testlog.NewTestLog(t, false, false)
			defer func() { tlog.Freeze() }()
			for _, d := range td.in {
				//may be set by previous test, which would throw us off
				d.array = nil
			}
			got := FindArrays(td.in)
			gots := got.String()
			wants := td.want.String()
			match := gots == wants
			if td.fail == match {
				t.Errorf("want\n%s\ngot\n%s", wants, gots)
			}
		})
	}
}

//func sizeMatch(a, b *Device) bool
func TestSizeMatch(t *testing.T) {
	a := &Device{devSize: 1234}
	b := &Device{devSize: 5678}
	c := &Device{devSize: 5679}
	if sizeMatch(a, b) {
		t.Error("a,b match")
	}
	if !sizeMatch(b, c) {
		t.Error("b,c mismatch")
	}
}
