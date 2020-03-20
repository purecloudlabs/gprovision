// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package common

type SerNumer interface {
	SerNum() string
}

type PlatInfoer interface {
	SerNumer
	DiagPorts() []int
	MACPrefixes() [][]byte
	IsPrototype() bool
	WANIndex() int
	PlatConfiger
}

type PlatConfiger interface {
	BiosConfigTool() string
	IpmiConfigTool() string
}
