// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

// Package netexport reads network config data from windows. The package
// github.com/purecloudlabs/gprovision/pkg/netexport can then be used to writes config files compatible with
// systemd-networkd.
//
// Requires Powershell, SaveRestore.ps1 (Intel). Some data is retrieved from
// the registry, while other data comes from the output of SaveRestore.ps1 or
// raw powershell commands.
package netexport
