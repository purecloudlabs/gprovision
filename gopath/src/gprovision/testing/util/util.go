// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

//Package util contains functions used by integ tests and by mage. To avoid
//chicken-and-egg problems with mage, its code cannot directly or indirectly
//import any packages with generated code. Otherwise, mage would be unable to
//compile and thus couldn't generate the code.
package util

import (
	"io"
	"log"
	"os"
	fp "path/filepath"
	"strings"

	"github.com/u-root/u-root/pkg/cpio"
	"github.com/u-root/u-root/pkg/uroot"
	"github.com/u-root/u-root/pkg/uroot/initramfs"
)

//looks for files in dirs like initramfs/ and initramfs_mfg/ in repo root
//adds them to a list for inclusion in cpio, format 'path/to/src:path/to/dest'
func FileList(d string) ([]string, error) {
	//repo root is one dir below GOPATH
	root := fp.Join(os.Getenv("INFRA_ROOT"), d)
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

//find repo root - from INFRA_ROOT env var, if set. otherwise search
//parents for a kernel config file and choose the first dir found.
func RepoRoot() (string, error) {
	rr := os.Getenv("INFRA_ROOT")
	if len(rr) > 0 {
		return rr, nil
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		cfg, err := fp.Glob(fp.Join(wd, "linux-*.config"))
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

// Helper function for mage, as dep puts u-root in the gprovision vendor dir
// where mage can't import it
func CreateInitramfs(outCpio, tmpdir string, baseArchive io.ReaderAt, files []string, existingInit bool) error {
	logger := log.New(os.Stderr, "initramfs: ", 0)
	out, err := initramfs.CPIO.OpenWriter(logger, outCpio, "", "")
	if err != nil {
		return err
	}
	err = uroot.CreateInitramfs(logger, uroot.Opts{
		TempDir:         tmpdir,
		SkipLDD:         true,
		BaseArchive:     cpio.Newc.Reader(baseArchive),
		ExtraFiles:      files,
		OutputFile:      out,
		UseExistingInit: existingInit,
	})
	return err
}
