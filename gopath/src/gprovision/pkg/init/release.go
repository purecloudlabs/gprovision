// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

// +build release

package init

import (
	"gprovision/pkg/log"
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

const (
	release = true
)

func handleEnvVars() {
	//check for and remove vars that affect go runtime
	badenv := []string{"GOGC=", "GODEBUG=", "GOMAXPROCS=", "GOTRACEBACK="}
	fixEnv := false
	current := os.Environ()
	var env []string
outer:
	for _, evar := range current {
		for _, bad := range badenv {
			if strings.HasPrefix(evar, bad) {
				fixEnv = true
				continue outer
			}
		}
		env = append(env, evar)
	}
	if fixEnv {
		log.Logf("fixing env vars...")
		//re-exec without offending var(s)
		unix.Exec(os.Args[0], os.Args, env)
	}
	commonEnvVars()
}

//no-op in release builds
func testOpts() {}
