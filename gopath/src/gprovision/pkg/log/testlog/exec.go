// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

// +build !release

package testlog

import (
	"fmt"
	"gprovision/pkg/log"
	"os/exec"
	"time"
)

//represents a Cmd in CmdMap
type Key string

//generates key for given command
func CmdKey(args []string) Key {
	k := ""
	for _, arg := range args {
		k += fmt.Sprintf("%s|", arg)
	}
	return Key(k)
}

//execution result
type Result struct {
	Res     string
	Success bool
}

//data for use with UseMappedCmdHijacker
type HijackerData struct {
	Result   Result        //if NoRun is false, this is updated with result on each run
	RunCount int           //number of times the command has been invoked
	NoRun    bool          //if true,returns already-stored Result
	Pause    time.Duration //in addition to any execution time, pause this long before returning
}

//map passed to UseMappedCmdHijacker
type CmdMap map[Key]HijackerData

//Using a map of commands, either record results or replay given results.
//Limitation: not able to return different results for different exec's of a
//given command.
func (tlog *TstLog) UseMappedCmdHijacker(m CmdMap) {
	log.Cmd = func(cmd *exec.Cmd) (res string, success bool) {
		tlog.t.Helper()
		key := CmdKey(cmd.Args)
		log.Logf("Running %v...", cmd.Args)
		data := m[key]
		data.RunCount++
		if data.NoRun {
			res, success = data.Result.Res, data.Result.Success
		} else {
			out, err := cmd.CombinedOutput()
			if err == nil {
				success = true
				res = string(out)
			} else {
				log.Logf("Running %v: error %s\noutput:\n%s\n", cmd.Args, err, string(out))
			}
			data.Result.Res, data.Result.Success = res, success
		}
		//update
		m[key] = data
		time.Sleep(data.Pause)
		return
	}
}
