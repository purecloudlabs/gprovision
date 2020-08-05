// Copyright (C) 2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

//Command currentBoot reads the current boot var
package main

import (
	"github.com/purecloudlabs/gprovision/pkg/hw/uefi"
	"github.com/purecloudlabs/gprovision/pkg/log"
)

//must run as root, as efi vars are not accessible otherwise
func main() {
	log.AddConsoleLog(0)
	log.FlushMemLog()
	v := uefi.ReadCurrentBootVar()
	if v == nil {
		log.Fatalf("unable to read var... are you root?")
		return // unreachable but keeps linter happy
	}
	log.Logf("%s", v)
	for _, element := range v.FilePathList {
		res, err := element.Resolver()
		if err != nil {
			log.Fatalf("%s", err)
		}
		log.Logf("%s", res.String())
	}
}
