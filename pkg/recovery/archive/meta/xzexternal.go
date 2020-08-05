// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

// +build linux,release

// above means linux AND release
// so this is only built for linux release builds (opposite of native.go)

package meta

import (
	"io"
)

// Decompress with xz executable. Faster than native. Returned function is for
// cleanup.
func unxzr(rdr io.Reader) (io.ReadCloser, func(), error) {
	return externalUnxz(rdr)
}
