// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

// +build !release

package common

type PlatMock struct {
	Diag         []int
	MPs          [][]byte
	Proto        bool
	Wan          int
	Ser          string
	BConf, IConf string
}

func (pm *PlatMock) DiagPorts() []int       { return pm.Diag }
func (pm *PlatMock) MACPrefixes() [][]byte  { return pm.MPs }
func (pm *PlatMock) IsPrototype() bool      { return pm.Proto }
func (pm *PlatMock) WANIndex() int          { return pm.Wan }
func (pm *PlatMock) SerNum() string         { return pm.Ser }
func (pm *PlatMock) BiosConfigTool() string { return pm.BConf }
func (pm *PlatMock) IpmiConfigTool() string { return pm.IConf }
