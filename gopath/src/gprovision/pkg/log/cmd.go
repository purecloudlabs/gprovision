// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package log

import (
	"os/exec"
)

type CommandFunc func(cmd *exec.Cmd) (res string, success bool)

//Wrapper for exec.Command(...).CombinedOutput(). If this is used, exec's can
//be mocked/tracked by testlog.
var Cmd CommandFunc = DefaultCmd

// Default impl of Cmd(); runs a command, capturing output, logging in the
// event of failure. On failure, returns "",false.
func DefaultCmd(cmd *exec.Cmd) (res string, success bool) {
	Logf("Running %v...", cmd.Args)
	out, err := cmd.CombinedOutput()
	if err == nil {
		success = true
		res = string(out)
		return
	}
	Logf("Running %v: error %s\noutput:\n%s\n", cmd.Args, err, string(out))
	return
}
