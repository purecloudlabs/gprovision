// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

// Package paths contains functions used by integ tests and by mage. NOTE:
// to avoid chicken-and-egg problems with mage, its code cannot directly or
// indirectly import any packages with generated code. Otherwise, mage would
// be unable to compile and thus couldn't generate the code.
package paths

import (
	"os"
	fp "path/filepath"
	"strings"
)

// Looks for files in dirs under build, such as initramfs/ and initramfs_mfg/.
// Adds them to a list for inclusion in cpio, format 'path/to/src:path/to/dest'
func FileList(d string) ([]string, error) {
	root := fp.Join(RepoRoot, "build", d)
	var flist []string
	err := fp.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			line := path + ":" + strings.TrimPrefix(path, root+"/")
			flist = append(flist, line)
		}
		return nil
	})
	return flist, err
}

// Find repo root - from INFRA_ROOT env var, if set. Otherwise
// search parents for a kernel config file and choose the first
// dir found.
func repoRoot() (string, error) {
	rr := os.Getenv("INFRA_ROOT")
	if len(rr) > 0 {
		return rr, nil
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		cfg, err := fp.Glob(fp.Join(wd, "build/linux-*.config"))
		if err != nil || len(cfg) > 0 {
			break
		}
		wd = fp.Dir(wd)
		if len(wd) < 2 {
			wd = ""
			break
		}
	}
	if wd == "" {
		return "", os.ErrInvalid
	}
	err = os.Setenv("INFRA_ROOT", wd)
	if err != nil {
		return "", err
	}
	return wd, nil
}

// Get the working dir location from env GPROV_WORKDIR if
// set, otherwise use a dir adjacent to repo root. Using
// one outside the repo root is done so that commands like
// 'go test ./...' run from repo root skip the work dir
// and are not slowed by scanning through the huge number
// of files/dirs in it.
func workDir() (string, error) {
	wd := os.Getenv("GPROV_WORKDIR")
	if len(wd) > 0 {
		return wd, nil
	}
	wd = fp.Join(fp.Dir(RepoRoot), fp.Base(RepoRoot)+"_work")
	err := os.Setenv("GPROV_WORKDIR", wd)
	if err != nil {
		return "", err
	}
	return wd, nil
}
