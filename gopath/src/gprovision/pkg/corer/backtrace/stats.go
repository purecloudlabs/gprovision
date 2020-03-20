// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package backtrace

import (
	"fmt"
	"gprovision/pkg/log"
	"io/ioutil"
	"os/exec"
	"syscall"
	"time"
)

func duration(tv syscall.Timeval) time.Duration {
	return time.Second*time.Duration(tv.Sec) + time.Microsecond*time.Duration(tv.Usec)
}
func rusageString(ru *syscall.Rusage) (usage string) {
	usage += fmt.Sprintf("  utime=%s", duration(ru.Utime).String())
	usage += fmt.Sprintf("\t stime=%s\n", duration(ru.Stime).String())
	usage += fmt.Sprintf("  minflt=%d", ru.Minflt)
	usage += fmt.Sprintf("\t majflt=%d\n", ru.Majflt)
	usage += fmt.Sprintf("  inblock=%d", ru.Inblock)
	usage += fmt.Sprintf("\t oublock=%d\n", ru.Oublock)
	usage += fmt.Sprintf("  nvcsw=%d", ru.Nvcsw)
	usage += fmt.Sprintf("\t nivcsw=%d\n", ru.Nivcsw)
	usage += fmt.Sprintf("  maxrss=%d kb\n", ru.Maxrss)
	//other fields are unmaintained/unset on linux
	return
}
func procRUsage(cmd *exec.Cmd) (out []byte) {
	if cmd.ProcessState == nil {
		log.Logln("attempted to print rusage for running process", cmd.Args)
		return
	}
	usage := cmd.ProcessState.SysUsage().(*syscall.Rusage)
	out = []byte(fmt.Sprintf("Resource usage for %s:\n%s\n", cmd.Args, rusageString(usage)))
	return
}

func captureMeminfo() []byte {
	m, _ := ioutil.ReadFile("/proc/meminfo")
	return m
}

func collectMemInfo(done chan struct{}, out chan []byte) {
	for {
		out <- captureMeminfo()
		select {
		case <-done:
			out <- captureMeminfo()
			close(out)
			return
		case <-time.After(time.Second):
		}
	}
}
