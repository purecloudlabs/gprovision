// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

//Package fileutil contains various utility functions useful for dealing with
//files and dirs.
package fileutil

import (
	"fmt"
	"gprovision/pkg/log"
	"io"
	"io/ioutil"
	"os"
	fp "path/filepath"
	"strings"
)

const (
	oneM = 1024.0 * 1024.0
)

//Computes size of dir and contents.
func DirSizeM(dir string) string {
	var size int64
	err := fp.Walk(dir, func(_ string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			size += info.Size()
		}
		return err
	})
	if err != nil {
		log.Logf("Error %s reading size of %s\n", err, dir)
		return "(unknown - error)"
	}
	if size == 0 {
		return "0 (no files)"
	}
	return ToMegs(size)
}

//Converts a size in bytes to megabytes; returns string with suffix 'MB'.
func ToMegs(size int64) string {
	return fmt.Sprintf("%.2fMB", float64(size)/oneM)
}

//Returns true if given path is a dir and is empty
func IsEmptyDir(dir string) bool {
	entries, err := ioutil.ReadDir(dir)
	return err == nil && len(entries) == 0
}

// Case-insensitive search of 'dir' for all of 'entries'. results will be an array
// the same size as entries, containing any matches - in the same order as entries.
// In the event of multiple files matching one of entries, the first seen is chosen.
func DirMatchCaseInsensitive(dir string, entries []string) (hit bool, results []string) {
	for i := range entries {
		entries[i] = strings.ToLower(entries[i])
	}
	results = make([]string, len(entries))
	fi, err := os.Open(dir)
	if err != nil {
		log.Logf("error %s opening %s for read", err, dir)
		return
	}
	defer fi.Close()
	names, err := fi.Readdirnames(8192) //8192 - arbitrary limit.
	if err != nil && err != io.EOF {
		log.Logf("error %s reading dir %s content", err, dir)
	}
	for _, name := range names {
		for i, entry := range entries {
			if strings.ToLower(name) == entry && len(results[i]) == 0 {
				hit = true
				results[i] = fp.Join(dir, name)
			}
		}
	}
	return
}

// Crude check that given path is not a glob. Does not understand escape
// sequences, so could return false positives.
func Globlike(p string) bool {
	return strings.Contains(p, "[]?*")
}

// Recursively find all files matching pattern in dir; return them and their
// total size. 'dir' must not be a glob pattern.
func ListFilesAndSize(dir, pattern string) (size int64, files []string) {
	if Globlike(dir) {
		log.Logf("Warning, dir passed to ListFilesAndSizes appears to be a glob pattern. This will not work. dir=%s", dir)
	}
	err := fp.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if info == nil || info.IsDir() {
			return err
		}
		match := false
		match, err = fp.Match(pattern, info.Name())
		if match {
			size += info.Size()
			files = append(files, path)
		}
		return err
	})
	if err != nil {
		log.Logf("Error %s reading size of %s\n", err, dir)
		return 0, nil
	}
	return
}
