// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

// Package altIdent allows reading, writing a file containing the platform's identity,
// for use as a fallback if the dmi data doesn't clearly identify the platform. This
// file is to be stored in the root of the recovery volume.
package altIdent

import (
	"io/ioutil"
	fp "path/filepath"
	"strings"

	"github.com/purecloudlabs/gprovision/pkg/log"
)

const (
	AltIdentFilename = "platform.ident"
)

func Read(recov string) (plat string) {
	fn := fp.Join(recov, AltIdentFilename)
	data, err := ioutil.ReadFile(fn)
	if err != nil {
		log.Logf("cannot read %s: %s", fn, err)
	}
	plat = strings.TrimSpace(string(data))
	log.Msgf("%s: %s", AltIdentFilename, plat)
	return
}

func Write(recov, plat string) {
	plat = strings.TrimSpace(plat)
	log.Logf("%s: %s", AltIdentFilename, plat)
	fn := fp.Join(recov, AltIdentFilename)
	err := ioutil.WriteFile(fn, []byte(plat), 0644)
	if err != nil {
		log.Logf("cannot write %s: %s", fn, err)
	}
}
