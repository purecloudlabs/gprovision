// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package lcd

import (
	"gprovision/pkg/log"
	"testing"
)

//will it crash if things are unset? should not
func TestNil(t *testing.T) {
	l := &LcdLog{}
	l.Finalize()
	l.AddEntry(log.LogEntry{})
	l.ForwardTo(nil)
	l.Ident()
	l.Next()
}
