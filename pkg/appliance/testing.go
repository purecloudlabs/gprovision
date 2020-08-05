// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

// +build !release

package appliance

import (
	"github.com/purecloudlabs/gprovision/pkg/log"
)

// To be used in testing. Allows assignment to normally-unassignable fields of Variant.
func TestSetup(i Variant_, mfg, prod, sku, serial string) (v *Variant) {
	v = &Variant{i: i, mfg: mfg, prod: prod, sku: sku, serial: serial}
	return
}

// Like TestSetup, but uses a codename and default data rather than Variant_
func TestSetupFrom(codename, mfg, prod, sku, serial string) *Variant {
	if variants == nil {
		//not loaded in tests
		j := getJson()
		err := loadJson(j)
		if err != nil {
			log.Logf("loading default json: %s", err)
			log.Fatalf("json error")
		}
	}
	v := Get(codename)
	if v == nil {
		return nil
	}
	return TestSetup(v.i, mfg, prod, sku, serial)
}
