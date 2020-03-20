// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package flags

import (
	"encoding/json"
	"fmt"
	"strings"
)

type Flag int

const (
	NA Flag = 0

	//ok to display message to end user
	EndUser Flag = 1 << (iota - 1) //iota increments with first ConstSpec in the const declaration, so subtract 1
	//logging a fatal error
	Fatal
	//do not write to local file log
	NotFile
	//do not write over the wire (not sure there'd be any use?)
	NotWire
)

func (f Flag) MarshalJSON() ([]byte, error) { return json.Marshal(f.String()) }
func (f Flag) String() string {
	switch f {
	case NA:
		return ""
	case EndUser:
		return "user"
	case Fatal:
		return "fatal"
	case NotFile:
		return "not file"
	case NotWire:
		return "not wire"
	}
	for _, bit := range []Flag{EndUser, Fatal, NotFile, NotWire} {
		if f&bit > 0 {
			return strings.Join([]string{bit.String(), (f &^ bit).String()}, "|")
		}
	}
	return fmt.Sprintf("0x%x", int(f))
}
