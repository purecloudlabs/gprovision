// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package fileutil

import (
	"bufio"
	"bytes"
	"io"
	"io/ioutil"
	"os"
	fp "path/filepath"
	"strings"
	"time"

	"github.com/purecloudlabs/gprovision/pkg/log"
)

var (
	xzId = [6]byte{0xfd, 0x37, 0x7a, 0x58, 0x5a, 0x00} // fd 37 7a 58 5a 00 -> xz archive
)

//return n bytes from beginning of file
func ReadHeader(fname string, n int64) (head []byte, err error) {
	f, err := os.Open(fname)
	if err != nil {
		return
	}
	defer f.Close()
	head, err = ioutil.ReadAll(io.LimitReader(f, n))
	if int64(len(head)) < n {
		return nil, io.ErrUnexpectedEOF
	}
	return
}

//checks for XZ header
func IsXZ(fname string) bool {
	head, err := ReadHeader(fname, int64(len(xzId)))
	if err != nil {
		log.Logf("failed to read head bytes from %s: %s", fname, err)
		return false
	}
	return bytes.Equal(head, xzId[:])
}

// Checks for XZ header and stream option byte indicating sha256
func IsXZSha256(fname string) bool {
	sigLen := int64(len(xzId))
	head, err := ReadHeader(fname, sigLen+2)
	if err != nil {
		return false
	}
	if !bytes.Equal(head[:sigLen], xzId[:]) {
		return false
	}
	//https://tukaani.org/xz/xz-file-format.txt section 2.1.1.2
	//8th byte of file is 0x0A for SHA256
	if head[sigLen] == 0 && head[sigLen+1] == 0x0a {
		return true
	}
	return false
}

// Renames old in same dir, using newPfx + random suffix (via os.TempFile)
func RenameUnique(old, newPfx string) (success bool) {
	f, err := ioutil.TempFile(fp.Dir(old), newPfx)
	if err != nil {
		log.Logf("error %s creating temp file for corrupt imaging history", err)
		err = os.Remove(old)
		if err != nil {
			log.Logf("error %s deleting corrupt history", err)
		}
		return false
	}
	newname := f.Name()
	f.Close()
	err = os.Remove(newname)
	if err != nil {
		log.Logf("error %s deleting temp file %s", err, newname)
	}
	err = os.Rename(old, newname)
	if err != nil {
		log.Logf("error %s renaming %s to %s", err, old, newname)
	}
	return err == nil
}

// WaitFor waits for a file to appear or times out. Returns true if file appears,
// false otherwise. Sleeps .1s between checks.
func WaitFor(path string, timeout time.Duration) (found bool) {
	stop := make(chan struct{})
	go func() {
		time.Sleep(timeout)
		close(stop)
	}()
	return WaitForChan(path, stop)
}

// WaitForChan is like WaitFor, but returns no later than when stop chan is closed
func WaitForChan(path string, stop chan struct{}) (found bool) {
	for {
		select {
		case <-stop:
			return
		case <-time.After(100 * time.Millisecond):
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			found = true
			break
		}
	}
	return
}

// ReadConfigLines reads a config file at the given path. Whitespace is
// stripped, as are comments (anything between # and \n). Individual lines
// are returned, up to maxLines.
func ReadConfigLines(path string, maxLines int) ([]string, error) {
	in, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer in.Close()
	var lines []string
	scanner := bufio.NewScanner(in)
	for scanner.Scan() {
		l := strings.TrimSpace(scanner.Text())
		if strings.Contains(l, "#") {
			l = strings.TrimSpace(strings.SplitN(l, "#", 2)[0]) //get rid of the comment
		}
		if len(l) == 0 {
			continue
		}
		lines = append(lines, l)
		if len(lines) == maxLines {
			log.Logf("ReadConfigLines: max lines (%d) read from %s", maxLines, path)
			break
		}
	}
	err = scanner.Err()
	if err != nil {
		return nil, err
	}
	return lines, nil
}
