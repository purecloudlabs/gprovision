// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package archive

import (
	"bytes"
	"gprovision/pkg/common/strs"
	"gprovision/pkg/fileutil"
	"gprovision/pkg/log/testlog"
	"gprovision/pkg/recovery/disk"
	"io/ioutil"
	"os"
	"os/exec"
	fp "path/filepath"
	"testing"
)

const (
	tenMegs = 1024 * 1024 * 10
)

func testUpdSort(t *testing.T, desc string, u, s []string, requireLenMatch bool) {
	sorted := sortUpdates(u, false)
	var sort1Pass = true
	if requireLenMatch && (len(sorted) != len(s)) {
		sort1Pass = false
	}
	for i := range s {
		if sorted[i] != s[i] {
			sort1Pass = false
			break
		}
	}
	if !sort1Pass {
		t.Logf("want\n%q\ngot\n%q", s, sorted)
		t.Errorf("sort test %s failed", desc)
	} else {
		t.Logf("sort test %s passes", desc)
	}
}

func TestUpdSort(t *testing.T) {
	unsorted1 := []string{
		"PRODUCT.Os.Platform-02.2015-02033.1593.gho",
		"PRODUCT.Os.Platform-02.2015-02-33.1593.gho",
		strs.ImgPrefix() + "2018-04-05.7037.upd",
		"PRODUCT.Os.Platform-02.2015-02-3.1593.gho",
		"PRODUCT.Os.Platform-02.2015-02-a3.1593.gho",
		strs.ImgPrefix() + "2018-04-05.7038.upd",
		"PRODUCT.OS3.Platform-02.2015-12-03.2000.upd",
		strs.ImgPrefix() + "2018-04-26.6940.upd",
		"PRODUCT.Os.Platform-02.2015-02-03.193o.gho",
		"PRODUCT.OtherOS.Platform-02.2016-09-02.4808.upd",
		"PRODUCT.Os.Platform-02.2015-05-33.1593.gho",
		"PRODUCT.OtherOS.Platform-02.2016-08-31.4798.upd",
		"PRODUCT.Os.Platform-02.2014-05-33.1593.gho",
		"PRODUCT.Os.Platform-02.2014-05-33.1594.gho",
	}
	sorted1 := []string{
		strs.ImgPrefix() + "2018-04-26.6940.upd",
		strs.ImgPrefix() + "2018-04-05.7038.upd",
		strs.ImgPrefix() + "2018-04-05.7037.upd",
		"PRODUCT.OtherOS.Platform-02.2016-09-02.4808.upd",
		"PRODUCT.OtherOS.Platform-02.2016-08-31.4798.upd",
		"PRODUCT.OS3.Platform-02.2015-12-03.2000.upd",
		"PRODUCT.Os.Platform-02.2015-05-33.1593.gho",
		"PRODUCT.Os.Platform-02.2015-02-33.1593.gho",
		"PRODUCT.Os.Platform-02.2014-05-33.1594.gho",
		"PRODUCT.Os.Platform-02.2014-05-33.1593.gho",
		/* order the bad updates appear in may vary, so don't care as long as they are last
		"PRODUCT.Os.Platform-02.2015-02-a3.1593.gho",
		"PRODUCT.Os.Platform-02.2015-02-03.193o.gho",
		"PRODUCT.Os.Platform-02.2015-02-3.1593.gho",
		"PRODUCT.Os.Platform-02.2015-02033.1593.gho",*/
	}
	testUpdSort(t, "New Test 1", unsorted1, sorted1, false)
}

//func validateExtractUpd(updPath string) (err error)
func TestValidationLowMem(t *testing.T) {
	tlog := testlog.NewTestLog(t, true, false)
	lowMemoryDevice = true
	err := testValidation(t)
	tlog.Freeze()
	if err != nil {
		t.Logf("log contents:\n%s\n", tlog.Buf.String())
	}
}

func TestValidationNormalMem(t *testing.T) {
	tlog := testlog.NewTestLog(t, true, false)
	lowMemoryDevice = false
	//use Tempfile to get a safe file name
	f, err := ioutil.TempFile("", "recovery_archive_test_decompress")
	if err != nil {
		t.Fatalf("%s", err)
	}
	f.Close()
	decompressBuf = f.Name()
	defer os.Remove(decompressBuf)
	if err = testValidation(t); err != nil {
		t.Errorf("test validation: %s", err)
	}
	tlog.Freeze()
	fi, err := os.Stat(decompressBuf)
	if err != nil {
		t.Fatalf("%s", err)
	}
	size := fi.Size()
	t.Logf("decompressed size: %d", size)
	if size != tenMegs {
		t.Logf("log contents:\n%s\n", tlog.Buf.String())
		t.Errorf("decompressed to wrong size: got %d, want %d. name=%s", size, tenMegs, decompressBuf)
	}
}

func testValidation(t *testing.T) (err error) {
	fname := create10McompressedDummy(t, true)
	defer os.Remove(fname)

	err = validateExtractUpd(fname)
	if err != nil {
		t.Errorf("%s", err)
	}
	return
}
func create10McompressedDummy(t *testing.T, shaChecksum bool) string {
	args := []string{}
	if shaChecksum {
		args = []string{"-C", "sha256"}
	}
	xz := exec.Command("xz", args...)
	xz.Stdin = bytes.NewBuffer(make([]byte, tenMegs))
	f, err := ioutil.TempFile("", "recovery_archive_test_dummy_xz")
	if err != nil {
		t.Fatalf("%s", err)
	}
	defer f.Close()
	xz.Stdout = f
	if err := xz.Run(); err != nil {
		t.Errorf("run xz: %s", err)
	}
	fi, err := f.Stat()
	if err != nil {
		t.Fatalf("%s", err)
	}
	t.Logf("size %d", fi.Size())
	return f.Name()
}

//func ApplyUpdate(target *disk.Filesystem)
//creates xz-compressed tarball for applyUpd to extract, checks output file name/size to verify decompress worked
func TestExtract(t *testing.T) {
	tlog := testlog.NewTestLog(t, true, false)
	lowMemoryDevice = true
	d, err := ioutil.TempDir("", "recovery_archive_testdir")
	if err != nil {
		t.Errorf("%s", err)
	}
	defer os.RemoveAll(d)
	f, err := ioutil.TempFile("", "tar-input")
	if err != nil {
		t.Fatalf("%s", err)
	}
	b := bytes.NewBuffer(make([]byte, tenMegs))
	if _, err := b.WriteTo(f); err != nil {
		t.Errorf("write test buffer: %s", err)
	}
	f.Close()
	defer os.Remove(f.Name())
	tar := exec.Command("tar", "c", f.Name())
	xz := exec.Command("xz", "-C", "sha256")
	buf := new(bytes.Buffer)
	xz.Stdin = buf
	tar.Stdout = buf
	g, err := ioutil.TempFile("", "recovery_archive_test_xz")
	if err != nil {
		t.Fatalf("%s", err)
	}
	defer g.Close()
	updateFullPath = g.Name()
	defer os.Remove(updateFullPath)
	xz.Stdout = g
	err = tar.Run()
	if err != nil {
		t.Logf("tar error: %s", err)
	}
	err = xz.Run()
	if err != nil {
		t.Logf("xz error: %s", err)
	}

	fs := disk.TestFilesystem(d)
	if !applyUpd(&fs) {
		t.Logf("log contents:\n%s\n", tlog.Buf.String())
		t.Errorf("applyUpd failed")
	}
	fi, err := os.Stat(fp.Join(d, f.Name()))
	if err != nil {
		t.Errorf("stat error: %s", err)
	}
	size := fi.Size()
	if size != tenMegs {
		t.Logf("log contents:\n%s\n", tlog.Buf.String())
		t.Errorf("decompressed to wrong size: got %d, want %d", size, tenMegs)
	}
}

// func IsXZSha256(fname string) bool
func TestIsXzSha256(t *testing.T) {
	with := create10McompressedDummy(t, true)
	defer os.Remove(with)
	if fileutil.IsXZSha256(with) != true {
		t.Errorf("IsXzSha256(): got false, want true")
	}
	without := create10McompressedDummy(t, false)
	defer os.Remove(without)
	if fileutil.IsXZSha256(without) != false {
		t.Errorf("IsXzSha256(): got true, want false")
	}
}
