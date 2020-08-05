// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package netexport

import (
	"os"
	"strings"

	inet "github.com/purecloudlabs/gprovision/pkg/net"
)

//file contains dummy functions to prevent compile errors, allow tests to run

func PersistentDNS(_ string) (_ string) {
	winOnly()
	return
}

func PersistentIPs(_ string) (_ []inet.IPNet, _ dhcp46) {
	winOnly()
	return
}

func winOnly() {
	if !strings.HasSuffix(os.Args[0], ".test") {
		panic("windows only")
	}
}
