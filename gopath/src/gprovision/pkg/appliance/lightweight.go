// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package appliance

import (
	"encoding/json"
	"gprovision/pkg/common/strs"
	"gprovision/pkg/log"
	"io/ioutil"
	fp "path/filepath"
)

// Functions in this file are so that apps in the image don't
// need to query smbios data etc etc (thus 'lightweight.go')
// factory restore writes the platform's info to a file
// (pjson, above) once the image has been written.

type PlatFacts struct {
	Variant_
	Mfg, Prod, SKU, Serial string
	//when writing in image, does it make sense to include macs?
}

// Use in factory restore to write the file. Path is the image root. Lcd type is
// untouched if ltype == NoLCD
func (v *Variant) WriteOut(path string, ltype LcdType) {
	file := fp.Join(path, strs.ConfDir(), "platform_facts.json")
	//make it possible to override lcd existence for prototyping
	if v.i.Prototype && ltype != NoLCD {
		current := v.i.Lcd
		defer func() { v.i.Lcd = current }()
		v.i.Lcd = ltype
	}
	v.write(file)
}

func (v *Variant) json() []byte {
	s := PlatFacts{
		Serial:   v.serial,
		Mfg:      v.mfg,
		Prod:     v.prod,
		SKU:      v.sku,
		Variant_: v.i,
	}
	data, err := json.Marshal(s)
	if err != nil {
		log.Logf("marshalling platform facts: %s", err)
		log.Fatalf("error creating platform_facts.json")
	}
	return data
}

func (v *Variant) write(file string) {
	data := v.json()
	err := ioutil.WriteFile(file, data, 0644)
	if err != nil {
		log.Fatalf("writing %s: %s", file, err)
	}
	log.Logf("wrote %s", file)
}

// Returns platform info. If appliance.Identify() has been called, uses its
// data. Otherwise loads info from platform_facts.json (see Pjson), written by
// factory restore.
func Read() *Variant {
	if identifiedVariant != nil {
		return identifiedVariant
	}
	return read(fp.Join(strs.ConfDir(), "platform_facts.json"))
}

func read(path string) (v *Variant) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		log.Logf("reading %s: %s", path, err)
		return
	}
	var s PlatFacts
	err = json.Unmarshal(data, &s)
	if err != nil {
		log.Logf("unmarshalling %s: %s", path, err)
	}
	v = new(Variant)
	v.i = s.Variant_
	v.serial = s.Serial
	v.mfg = s.Mfg
	v.prod = s.Prod
	v.sku = s.SKU
	return
}
