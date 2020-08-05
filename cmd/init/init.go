// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

// Command init replaces the sh script /init in an initramfs, doing the normal early
// userspace tasks. See github.com/purecloudlabs/gprovision/pkg/init for details.
package main

import (
	ini "github.com/purecloudlabs/gprovision/pkg/init"
	"github.com/purecloudlabs/gprovision/pkg/init/progress"
	"github.com/purecloudlabs/gprovision/pkg/log"
)

//in any binary with main.buildId string, it is set at compile time to $BUILD_INFO
var buildId string

func main() {
	//does nothing if argv[0]!=progress; never returns if it does match
	progress.MaybeStart()

	log.Logf("buildId: %s", buildId)
	ini.Init()
}
