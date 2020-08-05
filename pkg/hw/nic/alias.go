// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package nic

import (
	"io/ioutil"

	"github.com/purecloudlabs/gprovision/pkg/log"
)

//Set IfAlias on a nic. Unless overwrite is true, first checks that no alias is set.
func (nic Nic) SetAlias(alias string, overwrite bool) (changed bool) {
	if !overwrite {
		a := nic.GetAlias()
		if a != "" {
			return
		}
	}
	err := ioutil.WriteFile("/sys/class/net/"+nic.device+"/ifalias", []byte(alias), 0644)
	if err == nil {
		changed = true
	} else {
		log.Logf("setting %s alias: %s", nic.device, err)
	}
	return
}

//Returns a network interface's IfAlias. Returns an empty string if an error occurs or there is no alias.
func (nic Nic) GetAlias() string {
	data, err := ioutil.ReadFile("/sys/class/net/" + nic.device + "/ifalias")
	if err != nil {
		log.Logf("reading %s alias: %s", nic.device, err)
	}
	return string(data)
}
