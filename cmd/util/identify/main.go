// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

// Command identify identifies a hardware device from dmi data, using
// github.com/purecloudlabs/gprovision/pkg/appliance. Once frequently used, it now exists only for use in
// troubleshooting.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/purecloudlabs/gprovision/pkg/appliance"
)

var buildId string

func main() {
	var ipmi, ver bool
	flag.BoolVar(&ipmi, "ipmi", false, "does platform have ipmi?")
	flag.BoolVar(&ver, "v", false, "print version and exit")
	flag.Parse()
	if ver {
		fmt.Printf("build %s\n", buildId)
		os.Exit(0)
	}
	platform := appliance.Read()
	if platform == nil {
		fmt.Fprintf(os.Stderr, "unknown platform - no info available\n")
		os.Exit(1)
	}
	fmt.Printf("platform=%s\n", platform.FamilyName())
	if ipmi {
		fmt.Printf("ipmi=%t\n", platform.HasIPMI())
	}
}
