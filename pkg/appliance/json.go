// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package appliance

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/purecloudlabs/gprovision/pkg/log"
	"github.com/purecloudlabs/gprovision/pkg/net/xfer"
)

func LoadJson(url string) {
	log.Logf("loading appliance json from %s", url)
	data, err := xfer.GetFile(url)
	if err != nil {
		log.Fatalf(fmt.Sprintf("bad json url: %s", err))
	}
	err = loadJson(data)
	if err != nil {
		log.Fatalf(fmt.Sprintf("loadJson: unmarshal error %s", err))
	}
}

func loadJson(data []byte) (err error) {
	var loadStruct struct{ Variants []Variant_ } //necessary because on output, we wrap the Variant array
	err = json.Unmarshal(data, &loadStruct)
	if err == nil {
		variants = loadStruct.Variants
	}
	return
}

func DumpDescriptions() {
	m, err := json.MarshalIndent(variants, "  ", "  ")
	if err != nil {
		fmt.Printf("failed to marshal, err=%s\ndata=\n%#v\n", err, variants)
	}
	fmt.Printf("{\n  \"Variants\": %s\n}\n", m)
}

func (v *ValidateRDEnum) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	switch strings.ToLower(s) {
	case "usb":
		*v = ValidateUSB
	case "sata":
		*v = ValidateSATA
	case "none":
		*v = NoValidation
	case "9p":
		*v = Validate9P
	default:
		log.Logf("unrecognized value for %T, assuming usb: %s", v, s)
		*v = ValidateUSB
	}
	return nil
}
func (v ValidateRDEnum) MarshalJSON() ([]byte, error) {
	var s string
	switch v {
	case ValidateUSB:
		s = "usb"
	case ValidateSATA:
		s = "sata"
	case NoValidation:
		s = "none"
	case Validate9P:
		s = "9p"
	default:
		return nil, fmt.Errorf("failed to marshal %T value %#v", v, v)
	}
	return json.Marshal(s)
}

func (l *LocateRDEnum) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	switch strings.ToLower(s) {
	case "bylabel":
		*l = LocateByLabel
	case "9p":
		*l = Locate9PVirt
	default:
		log.Logf("unrecognized value for %T, assuming bylabel: %s", l, s)
		*l = LocateByLabel
	}
	return nil
}
func (l LocateRDEnum) MarshalJSON() ([]byte, error) {
	var s string
	switch l {
	case LocateByLabel:
		s = "byLabel"
	case Locate9PVirt:
		s = "9p"
	default:
		return nil, fmt.Errorf("failed to marshal %T value %#v", l, l)
	}
	return json.Marshal(s)
}

func (l *LcdType) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	switch s {
	case "631":
		*l = Cfa631
	case "635":
		*l = Cfa635
	case "none":
		*l = NoLCD
	default:
		log.Logf("unrecognized value for %T, assuming none: %s", l, s)
		*l = NoLCD
	}
	return nil
}
func (l LcdType) MarshalJSON() ([]byte, error) {
	var s string
	switch l {
	case Cfa631:
		s = "631"
	case Cfa635:
		s = "635"
	case NoLCD:
		s = "none"
	default:
		return nil, os.ErrInvalid
	}
	return json.Marshal(s)
}
