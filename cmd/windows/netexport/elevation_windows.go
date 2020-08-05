// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package main

import (
	"unsafe"

	"github.com/purecloudlabs/gprovision/pkg/log"

	win "golang.org/x/sys/windows"
)

/* https://msdn.microsoft.com/en-us/library/windows/desktop/bb530717(v=vs.85).aspx
typedef struct _TOKEN_ELEVATION {
  DWORD TokenIsElevated;
} TOKEN_ELEVATION, *PTOKEN_ELEVATION;
*/
type tokenElevation struct {
	elevated uint32
}

func checkElevation() {
	t, err := win.OpenCurrentProcessToken()
	if err != nil {
		panic(err)
	}
	defer t.Close()
	//uses of getInfo() in sys/windows succeed with a size of 50 which seems arbitrary, but TokenElevation only works when it's exactly 4
	p, err := getInfo(t, win.TokenElevation, 4)
	if err != nil {
		panic(err)
	}
	if (*tokenElevation)(p).elevated == 0 {
		log.Msgln("NOT running with elevated privileges")
	} else {
		log.Logln("running with elevated privileges")
	}
}

// getInfo retrieves a specified type of information about an access token.
// copied from golang.org/x/sys/windows
func getInfo(t win.Token, class uint32, initSize int) (unsafe.Pointer, error) {
	n := uint32(initSize)
	for {
		b := make([]byte, n)
		e := win.GetTokenInformation(t, class, &b[0], uint32(len(b)), &n)
		if e == nil {
			return unsafe.Pointer(&b[0]), nil
		}
		if e != win.ERROR_INSUFFICIENT_BUFFER {
			return nil, e
		}
		if n <= uint32(len(b)) {
			return nil, e
		}
	}
}
