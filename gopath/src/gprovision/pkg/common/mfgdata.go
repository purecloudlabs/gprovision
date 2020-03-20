// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package common

//Data needed for Stasher, that comes from mfgdata retrieved from server.
type StashData interface {
	//endpoint that may be used by Credentialer
	CredEP() string
	//list of files needed by Stasher
	StashFileList() []TransferableVerifiableFile
}

type Credentialer interface {
	//set endpoint to use
	SetEP(ep string)
	//get credentials for unit with given id
	GetCredentials(ident string) Credentials
}

//set of credentials for a given unit
type Credentials struct {
	OS, BIOS, IPMI string
}
