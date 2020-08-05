// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package netexport

import (
	"encoding/json"
	"net"
	"strings"
)

//wrapper to ensure the MAC gets converted to a JSON string correctly
type StringyMac struct {
	net.HardwareAddr
}

func (m StringyMac) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.String())
}
func (m *StringyMac) UnmarshalJSON(data []byte) error {
	mac := strings.Trim(string(data), `"`)
	addr, err := net.ParseMAC(mac)
	if err == nil {
		m.HardwareAddr = addr
	}
	return err
}
