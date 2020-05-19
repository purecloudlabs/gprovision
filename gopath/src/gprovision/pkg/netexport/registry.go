// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package netexport

import (
	"net"
)

//file contains functions that aren't dependent on windows, and can thus be tested on linux

func maskFromString(m string) net.IPMask {
	sub := net.ParseIP(m)
	mask := net.IPv4Mask(sub[12], sub[13], sub[14], sub[15])
	//fmt.Printf("%s -> %s -> %s\n", m, sub.String(), mask.String())
	return mask
}
