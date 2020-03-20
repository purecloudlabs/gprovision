// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package netexport

import (
	"encoding/binary"
	"fmt"
	"net"
)

//file contains functions that aren't dependent on windows, and can thus be tested on linux

//decode GUID found in REG_BINARY, return as uppercase string
func guidStrFromRegBin(binGuid []byte) string {
	if len(binGuid) != 16 {
		panic("bad len")
	}
	le := binary.LittleEndian
	data1 := le.Uint32(binGuid[:4])
	data2 := le.Uint16(binGuid[4:6])
	data3 := le.Uint16(binGuid[6:8])
	return fmt.Sprintf("{%08X-%04X-%04X-%04X-%012X}", data1, data2, data3, binGuid[8:10], binGuid[10:])
}

func maskFromString(m string) net.IPMask {
	sub := net.ParseIP(m)
	mask := net.IPv4Mask(sub[12], sub[13], sub[14], sub[15])
	//fmt.Printf("%s -> %s -> %s\n", m, sub.String(), mask.String())
	return mask
}
