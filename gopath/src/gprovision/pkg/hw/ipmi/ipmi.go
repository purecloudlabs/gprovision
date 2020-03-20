// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

//Package ipmi interacts with the platform BMC. User privileges can be checked,
//passwords changed, MACs retrieved, etc.
package ipmi

import (
	"gprovision/pkg/log"
	"os/exec"
)

func Available() bool {
	log.Msgf("ipmi info is not implemented!")
	return false

	//_, err := os.Stat("/dev/ipmi0")
	//return err == nil
}

func Versions() (ipmi, fru, sdr, me string) {
	//FIXME
	out, err := exec.Command("ipmiutil", "--versions").CombinedOutput()
	if err != nil {
		return
	}
	ipmi = string(out)
	return
}
