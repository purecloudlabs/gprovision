// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

// Package status can be used to query service status, stop/start services, etc.
// Shells out to 'systemctl'. Defaults to the system's service manager; use
// UserContext() for user services.
package status

import (
	"gprovision/pkg/log"
	"io/ioutil"
	"os/exec"
	"strings"
)

//Methods called on this operate in system context.
func SystemContext() (ctx sysdCtx) {
	return
}

//Methods called on this operate in user context.
func UserContext() (ctx sysdCtx) {
	ctx.user = true
	return
}

//True if sysctl reports service is active.
func IsActive(service string) bool { return SystemContext().IsActive(service) }
func (ctx sysdCtx) IsActive(service string) bool {
	return ctx.sysctlCmdBool("is-active", service)
}

//True if sysctl reports service is failed.
func IsFailed(service string) bool { return SystemContext().IsFailed(service) }
func (ctx sysdCtx) IsFailed(service string) bool {
	return ctx.sysctlCmdBool("is-failed", service)
}

//Start a service, returning any error.
func Start(service string) error { return SystemContext().Start(service) }
func (ctx sysdCtx) Start(service string) error {
	return ctx.sysctlCmdErr("start", service)
}

//Stop a service, returning any error.
func Stop(service string) error { return SystemContext().Stop(service) }
func (ctx sysdCtx) Stop(service string) error {
	return ctx.sysctlCmdErr("stop", service)
}

//List any services that are failed.
func Failed() []string { return SystemContext().Failed() }
func (ctx sysdCtx) Failed() (list []string) {
	sysctl := exec.Command("systemctl", ctx.arg(), "--failed", "--no-legend")
	out, err := sysctl.Output()
	if err != nil {
		log.Logf("error %s running %v", err, sysctl.Args)
		return nil
	}
	lines := strings.Split(string(out), "\n")
	for _, l := range lines {
		split := strings.Split(l, " ")
		svc := strings.TrimSpace(split[0])
		if len(svc) > 0 {
			list = append(list, svc)
		}
	}
	return
}

//Shutdown.
func Poweroff() error {
	return exec.Command("systemctl", "--system", "poweroff", "-q").Run()
}

//Reboot.
func Reboot() error {
	return exec.Command("systemctl", "--system", "reboot", "-q").Run()
}

//Is the current init system systemd?
func IsSystemd() bool {
	data, err := ioutil.ReadFile("/proc/1/cmdline")
	if err != nil {
		log.Logf("error determining init system: %s", err)
	}
	return strings.Contains(string(data), "systemd")
}

type sysdCtx struct {
	user bool
}

func (ctx sysdCtx) arg() (ctxArg string) {
	if ctx.user {
		ctxArg = "--user"
		return
	}
	ctxArg = "--system"
	return
}
func (ctx sysdCtx) sysctlCmdBool(cmd, arg string) bool {
	err := ctx.sysctlCmdErr(cmd, arg)
	if err == nil {
		return true
	}
	_, ok := err.(*exec.ExitError)
	if ok {
		//process exited with non-zero error code
		return false
	}
	log.Logf("error %s running systemctl with cmd=%s, arg=%s", err, cmd, arg)
	return false
}
func (ctx sysdCtx) sysctlCmdErr(cmd, arg string) error {
	sysctl := exec.Command("systemctl", ctx.arg(), cmd, "-q", arg)
	return sysctl.Run()
}
