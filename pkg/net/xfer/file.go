// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

//Package xfer handles robust file transfers, primarily for use in mfg process.
package xfer

import (
	"crypto/sha1"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	fp "path/filepath"
	"strings"
	"syscall"
	"time"

	futil "github.com/purecloudlabs/gprovision/pkg/fileutil"
	"github.com/purecloudlabs/gprovision/pkg/log"
)

//retrieves file, either on local fs or via http/https
func GetFile(url string) (content []byte, err error) {
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		log.Logf("downloading %s", url)
		var res *http.Response
		res, err = http.Get(url)
		if err == nil {
			defer res.Body.Close()
			content, err = ioutil.ReadAll(res.Body)
		}
	} else {
		content, err = ioutil.ReadFile(url)
	}
	return
}

type VerifiableFile interface {
	Verify() error
}
type TransferrableFile interface {
	Get() error
}

var _ TransferrableFile = &TVFile{}
var _ VerifiableFile = &TVFile{}

type TVFile struct {
	Dest             string
	Src              string
	Sha1             string
	useIntermediate  bool //copy to temporary/staging location first
	intermediateFile string
	finalized        bool
	mode             os.FileMode
}

func (tvf *TVFile) Basename() string {
	return fp.Base(tvf.Src)
}

/* When copying over network, write to intermediate location and verify
   hash before writing to final dest. Useful when dest is slow (usb flash)
   to avoid rewrites in the face of checksum failures.
*/
func (tvf *TVFile) UseIntermediateDir(dir string) {
	tvf.useIntermediate = true
	f, err := ioutil.TempFile(dir, tvf.Basename())
	if err != nil {
		log.Logf("failed to create temp file for %s: %s", tvf.Basename(), err)
		tvf.intermediateFile = fp.Join(dir, tvf.Basename())
		return
	}
	tvf.intermediateFile = f.Name()
	f.Close()
}

//return path of temp file we downloaded to
func (tvf *TVFile) GetIntermediate() string {
	if !tvf.useIntermediate {
		panic("intermediate location not set")
	}
	return tvf.intermediateFile
}

//Verify SHA1 in most recent location (intermediate or final)
func (tvf *TVFile) Verify() (err error) {
	fname := tvf.Dest
	if tvf.useIntermediate && !tvf.finalized {
		fname = tvf.intermediateFile
	}
	/* use fsync to ensure file is written to media */
	var f *os.File
	f, err = os.Open(fname)
	if err != nil {
		return
	}
	defer f.Close()
	log.Logf("sync %s before verifying...", fname)
	err = syscall.Fsync(int(f.Fd())) //convert Fd() (type uintptr) to int so Fsync() can convert it back. grr.
	if err != nil {
		return
	}
	return verify(fname, tvf.Sha1)
}
func verify(fname, sha string) (err error) {
	f, err := os.Open(fname)
	if err != nil {
		return
	}
	defer f.Close()

	hasher := sha1.New()
	_, err = io.Copy(hasher, f)
	if err != nil {
		return
	}

	computed := fmt.Sprintf("%x", hasher.Sum(nil))
	if sha != computed {
		err = fmt.Errorf("bad sha1.\nwant %s\ngot  %s\n", sha, computed)
	}
	return
}

//get a file from url, verifying integrity with sha1
func (tvf *TVFile) Get() (err error) {
	dest := tvf.Dest
	if tvf.useIntermediate {
		dest = tvf.intermediateFile
	}
	err = os.MkdirAll(fp.Dir(dest), 0777)
	if err != nil {
		log.Logf("failed to create dir: %s")
	}

	if !strings.HasPrefix(tvf.Src, "http://") && !strings.HasPrefix(tvf.Src, "https://") {
		return fmt.Errorf("Error: url '%s' must be http or https", dest)
	}
	log.Logf("downloading %s", tvf.Basename())

	var res *http.Response
	res, err = http.Get(tvf.Src)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if tvf.mode == 0 {
		tvf.mode = 0666
	}
	dst, err := os.OpenFile(dest, os.O_RDWR|os.O_CREATE|os.O_TRUNC, tvf.mode)
	if err != nil {
		return err
	}
	defer dst.Close()

	writeDone := make(chan struct{})
	go futil.ShowProgress(writeDone, "Downloading", dest)
	defer close(writeDone)

	_, err = io.Copy(dst, res.Body)
	if err != nil {
		return
	}
	return verify(dest, tvf.Sha1)
}

//set mode with which file is to be created
func (tvf *TVFile) Mode(m os.FileMode) {
	tvf.mode = m
}

//if file is in intermediate location, moves to final location
func (tvf *TVFile) Finalize() (err error) {
	current := tvf.Dest
	if tvf.useIntermediate && !tvf.finalized {
		current = tvf.intermediateFile
	}
	_, err = os.Stat(current)
	if err != nil {
		panic(fmt.Sprintf("error stat'ing %s: %s", current, err))
	}
	if tvf.useIntermediate {
		err = os.MkdirAll(fp.Dir(tvf.Dest), 0777)
		if err != nil {
			log.Logf("failed to create dir: %s", err)
		}
		err = futil.CopyFile(tvf.intermediateFile, tvf.Dest, 0)
		if err == nil {
			e := os.Remove(tvf.intermediateFile)
			if e != nil {
				log.Logf("failed to remove temp file %s: %s", tvf.intermediateFile, e)
			}
		}
	}
	tvf.finalized = (err == nil)
	return
}

//like Get() but retries with exponential backoff
func (tvf *TVFile) GetWithRetry() error {
	sleepTime := 10 * time.Second
	success := false
	for retries := 5; retries > 0; retries-- {
		err := tvf.Get()
		if err == nil {
			log.Logf("valid checksum for %s", tvf.Basename())
			success = true
			break
		}
		log.Msgf("failed to retrieve %s", fp.Base(tvf.Src))
		log.Logf("retrieval error %s", err)
		if retries > 0 {
			log.Msgf("sleep %s, retry", sleepTime)
			time.Sleep(sleepTime)
			sleepTime *= 2
		}
	}
	if !success {
		return fmt.Errorf("gave up retrieving %s", tvf.Src)
	}
	return nil
}
