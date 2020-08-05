// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package appliance

import (
	"testing"

	"github.com/purecloudlabs/gprovision/pkg/hw/dmi"
	"github.com/purecloudlabs/gprovision/pkg/log/testlog"
)

func TestIdentify(t *testing.T) {
	err := loadJson([]byte(aj_default))
	if err != nil {
		t.Error(err)
	}
	testIdent(t, identMocks)
}

func testIdent(t *testing.T, mocks []identTestData) {
	for _, mock := range mocks {
		t.Run(mock.name, func(t *testing.T) {
			tlog := testlog.NewTestLog(t, true, false)
			dmi.TestingMock(mock.sm, mock.tm)
			v := ReIdentify()
			if v == nil && !mock.fail {
				t.Errorf("identify failed for %s/%s", mock.name, mock.codename)
			} else if v != nil && mock.fail {
				t.Errorf("identify should fail for %s/%s but did not\n%+v", mock.name, mock.codename, v)
			} else {
				t.Logf("identify result is as expected for %s/%s: fail=%t", mock.name, mock.codename, mock.fail)
			}
			if v != nil {
				if v.DeviceCodeName() != mock.codename {
					t.Errorf("%s: want codename %s, got  %s", mock.name, mock.codename, v.DeviceCodeName())
				}
				if v.SerNum() != mock.serial {
					t.Errorf("%s: want serial %s, got %s", mock.name, mock.serial, v.SerNum())
				}
			}
			if t.Failed() {
				tlog.Freeze()
				l := tlog.Buf.String()
				t.Logf("log content for %s/%s:\n%s\n", mock.name, mock.codename, l)
			}
		})
	}
}

type identTestData struct {
	sm                     dmi.DmiStrMap
	tm                     dmi.DmiTypeMap
	codename, serial, name string
	cpus                   string
	fail                   bool
}

var identMocks = []identTestData{
	{
		name:     "unknown",
		sm:       dmi.DmiStrMap{},
		tm:       dmi.DmiTypeMap{},
		codename: "unknown",
		serial:   "",
		fail:     true,
	},
	{
		name: "qemu-normal",
		sm: dmi.DmiStrMap{
			"bios-vendor":             "SeaBIOS",
			"bios-version":            "1.10.2-1.fc27",
			"bios-release-date":       "04/01/2014",
			"system-manufacturer":     "GPROV_QEMU",
			"system-product-name":     "mfg_test",
			"system-version":          "pc-i440fx-2.11",
			"system-serial-number":    "QEMU01234",
			"system-uuid":             "Not Settable",
			"baseboard-manufacturer":  "",
			"baseboard-product-name":  "",
			"baseboard-version":       "",
			"baseboard-serial-number": "",
			"baseboard-asset-tag":     "",
			"chassis-manufacturer":    "QEMU",
			"chassis-type":            "Other",
			"chassis-version":         "pc-i440fx-2.11",
			"chassis-serial-number":   "Not Specified",
			"chassis-asset-tag":       "Not Specified",
			"processor-family":        "Other",
			"processor-manufacturer":  "QEMU",
			"processor-version":       "pc-i440fx-2.11",
			"processor-frequency":     "2000 MHz",
		},
		tm: dmi.DmiTypeMap{
			1: []byte(`Handle 0x0100, DMI type 1, 27 bytes
System Information
	Manufacturer: GPROV_QEMU
	Product Name: mfg_test
	Version: pc-i440fx-2.11
	Serial Number: QEMU01234
	UUID: Not Settable
	Wake-up Type: Power Switch
	SKU Number: Not Specified
	Family: Not Specified
`),
		},
		codename: "QEMU-mfg-test",
		serial:   "QEMU01234",
	},
	{
		name: "qemu-all-unique",
		sm: dmi.DmiStrMap{
			"bios-vendor":             "biosvnd",
			"bios-version":            "biosver",
			"bios-release-date":       "biosrd",
			"system-manufacturer":     "GPROV_QEMU",
			"system-product-name":     "mfg_test",
			"system-version":          "sysver",
			"system-serial-number":    "QEMU01234",
			"system-uuid":             "sysuuid",
			"baseboard-manufacturer":  "bm",
			"baseboard-product-name":  "bpn",
			"baseboard-version":       "bv",
			"baseboard-serial-number": "bsn",
			"baseboard-asset-tag":     "bat",
			"chassis-manufacturer":    "cm",
			"chassis-type":            "ct",
			"chassis-version":         "cv",
			"chassis-serial-number":   "csn",
			"chassis-asset-tag":       "cat",
			"processor-family":        "pf",
			"processor-manufacturer":  "pm",
			"processor-version":       "pv",
		},
		tm: dmi.DmiTypeMap{
			1: []byte("Handle 0x0\nSKU Number: Not Specified\n"),
		},
		codename: "QEMU-mfg-test",
		serial:   "QEMU01234",
	},
	{
		name: "test-cpu1",
		sm: dmi.DmiStrMap{
			"system-manufacturer":  "cputest",
			"system-product-name":  "cputest",
			"system-serial-number": "abcd",
			"processor-version":    "someVersion",
		},
		tm: dmi.DmiTypeMap{
			1: []byte("Handle 0x0\nSKU Number: Not Specified\n"),
		},
		codename: "cputest1",
		serial:   "abcd",
	},
	{
		name: "test-cpu2",
		sm: dmi.DmiStrMap{
			"system-manufacturer":  "cputest",
			"system-product-name":  "cputest",
			"system-serial-number": "efgh",
			"processor-version":    "someOtherVersion",
		},
		tm: dmi.DmiTypeMap{
			1: []byte("Handle 0x0\nSKU Number: Not Specified\n"),
		},
		codename: "cputest2",
		serial:   "efgh",
	},
	{
		name: "test-badcpu",
		sm: dmi.DmiStrMap{
			"system-manufacturer":  "cputest",
			"system-product-name":  "cputest",
			"system-serial-number": "efgh",
			"processor-version":    "yetAnotherVersion",
		},
		tm: dmi.DmiTypeMap{
			1: []byte("Handle 0x0\nSKU Number: Not Specified\n"),
		},
		fail: true,
	},
}

//func IdentifyWithFallback(fallback func() string) *Variant
func TestFallback(t *testing.T) {
	dmi.TestingMock(dmi.DmiStrMap{}, dmi.DmiTypeMap{})
	identifiedVariant = nil

	v, err := IdentifyWithFallback(func() (string, error) { return "QEMU-mfg-test", nil })
	if err != nil {
		t.Error(err)
	}
	if v == nil {
		t.Error("platform is nil")
		return
	}
	if v.DeviceCodeName() != "QEMU-mfg-test" {
		t.Errorf("got %+v\n", v)
	}
}

//func (v *Variant_) processorMatch() bool
func TestProcessorMatch(t *testing.T) {
	gold := "Intel(R) Xeon(R) Gold 5118 CPU @ 2.30GHz"
	old := "Intel(R) Xeon(R) CPU E5-2418L v2 @ 2.00GHz"
	ns := "Not Specified"
	cng := ns + "\n" + gold
	cgn := gold + "\n" + ns
	cgg := gold + "\n" + gold
	con := old + "\n" + ns
	mocks := []identTestData{
		{
			name: "cgn",
			sm:   dmi.DmiStrMap{"processor-version": cgn},
			cpus: cgn,
		},
		{
			name: "cng",
			sm:   dmi.DmiStrMap{"processor-version": cng},
			cpus: cng,
		},
		{
			name: "cgn-swap",
			sm:   dmi.DmiStrMap{"processor-version": cng},
			cpus: cgn,
		},
		{
			name: "cgg",
			sm:   dmi.DmiStrMap{"processor-version": cgg},
			cpus: cgg,
		},
		{
			name: "con",
			sm:   dmi.DmiStrMap{"processor-version": con},
			cpus: con,
		},
		{
			name: "old",
			sm:   dmi.DmiStrMap{"processor-version": old},
			cpus: old,
		},
		{
			name: "gold",
			sm:   dmi.DmiStrMap{"processor-version": gold},
			cpus: gold,
		},
		{
			name: "cgg-fail",
			sm:   dmi.DmiStrMap{"processor-version": cgg},
			cpus: cgn,
			fail: true,
		},
		{
			name: "cgg-fail2",
			sm:   dmi.DmiStrMap{"processor-version": cgg},
			cpus: gold,
			fail: true,
		},
		{
			name: "gold-fail",
			sm:   dmi.DmiStrMap{"processor-version": gold},
			cpus: old,
			fail: true,
		},
		{
			name: "old-fail",
			sm:   dmi.DmiStrMap{"processor-version": old},
			cpus: gold,
			fail: true,
		},
	}
	for _, m := range mocks {
		t.Run(m.name, func(t *testing.T) {
			tlog := testlog.NewTestLog(t, true, false)
			dmi.TestingMock(m.sm, nil)
			v := &Variant_{CPU: m.cpus}
			match := v.processorMatch()
			if match == m.fail {
				t.Error("mismatch")
			}
			tlog.Freeze()
			if t.Failed() {
				t.Log(tlog.Buf.String())
			}
		})
	}
}
