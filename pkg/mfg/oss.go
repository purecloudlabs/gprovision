// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package mfg

import (
	"github.com/purecloudlabs/gprovision/pkg/oss/frd"
	"github.com/purecloudlabs/gprovision/pkg/oss/pblog"
	"github.com/purecloudlabs/gprovision/pkg/oss/stash"
)

func init() {
	pblog.UseRLoggerSetup()
	pblog.UseRKeeper()
	stash.UseImpl()
	frd.UseImpl()
}
