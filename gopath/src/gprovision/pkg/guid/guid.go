// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

// Package guid handles uuid's encoded in the mixed-endianness format used by
// microsoft and uefi. For normal uuid-related functionality, use a different
// package - such as github.com/google/uuid .
package guid

import (
	"strings"

	"github.com/google/uuid"
)

//A mixed-endianness guid, as used by MS and UEFI.
type MixedGuid [16]byte

//Converts MixedGuid to a uuid.UUID
func (m MixedGuid) ToStdEnc() (u uuid.UUID) {
	u[0], u[1], u[2], u[3] = m[3], m[2], m[1], m[0]
	u[4], u[5] = m[5], m[4]
	u[6], u[7] = m[7], m[6]
	copy(u[8:], m[8:])
	return
}

//Converts uuid.UUID to MixedGuid
func FromStdEnc(u uuid.UUID) (m MixedGuid) {
	m[0], m[1], m[2], m[3] = u[3], u[2], u[1], u[0]
	m[4], m[5] = u[5], u[4]
	m[6], m[7] = u[7], u[6]
	copy(m[8:], u[8:])
	return
}

// MSStr returns uppercase representation, in curly braces ("ms registry format")
func MSStr(u uuid.UUID) string {
	return "{" + strings.ToUpper(u.String()) + "}"
}

// MSStrFromRegBin decodes GUID found in REG_BINARY, return as uppercase string
// in {}. Used when extracting values from the windows registry.
func MSStrFromRegBin(binGuid []byte) string {
	if len(binGuid) != 16 {
		panic("bad len")
	}
	var m MixedGuid
	copy(m[:], binGuid)
	return MSStr(m.ToStdEnc())
}
