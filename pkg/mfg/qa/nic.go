// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package qa

import (
	"net"
	"strings"

	"github.com/purecloudlabs/gprovision/pkg/common/strs"
	"github.com/purecloudlabs/gprovision/pkg/hw/nic"
	"github.com/purecloudlabs/gprovision/pkg/log"
)

func (s *Specs) GetNicInfo() {
	interfaces, err := net.Interfaces()
	if err != nil {
		log.Logf("error reading interfaces: %s", err)
	}
	var nics []net.Interface
	//filter out virtual interfaces
	for _, i := range interfaces {
		if strings.HasPrefix(i.Name, "e") && (i.Flags&net.FlagLoopback) == 0 {
			nics = append(nics, i)
		}
	}
	var prefixedMacs []net.HardwareAddr
	s.TotalNics = len(nics)
	pfx := strs.MacOUI()
	for _, n := range nics {
		if strings.HasPrefix(n.HardwareAddr.String(), pfx) {
			prefixedMacs = append(prefixedMacs, n.HardwareAddr)
		}
	}
	s.NumOUINics = len(prefixedMacs)

	//create sorted list of hardware addresses for sequentiality test
	macs := nic.SortableMacs(prefixedMacs)
	macs.Sort()
	s.OUINicsSequential = macs.Sequential()
}
