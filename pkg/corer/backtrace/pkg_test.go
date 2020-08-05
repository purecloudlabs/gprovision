// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package backtrace

import (
	"archive/zip"
	"bytes"
	"crypto/sha1"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/purecloudlabs/gprovision/pkg/corer/testhelper"
	"github.com/purecloudlabs/gprovision/pkg/log/testlog"
)

//func findExeByName(corePath string) (exe string)
func TestFindExeByName(t *testing.T) {
	data := []struct{ tstName, coreName, exe string }{
		//[imagename_]exe-pid-uid-gid-sig-time.core
		{"goodImgName", "python_Main-p-u-g-s-t.core", "python"},
		{"missingImgName", "dummyimg_Main-p-u-g-s-t.core", ""},
		{"missingExe", "dummyexe-p-u-g-s-t.core", ""},
		{"goodExe", "true-p-u-g-s-t.core", "true"},
		{"goodExeCorePath", "/path/to/true-p-u-g-s-t.core", "true"},
	}
	for _, item := range data {
		t.Run(item.tstName, func(t *testing.T) {
			tlog := testlog.NewTestLog(t, true, false)
			found := findExeByName(item.coreName)
			tlog.Freeze()
			out := tlog.Buf.String()
			exe := ""
			if item.exe != "" {
				exe, _ = exec.LookPath(item.exe)
				if exe == "" {
					t.Error("exe not found")
				}
			}
			if found != exe {
				t.Errorf("got %s, want %s (%s)\n", found, exe, item.exe)
			}
			if t.Failed() {
				t.Logf("log content:\n%s\n", out)
			}
		})
	}
}

//func parseCoreName(corePath string)(exe string)
func TestParseCoreName(t *testing.T) {
	data := []struct{ tstName, coreName, exe string }{
		//[imagename_]exe-pid-uid-gid-sig-time.core
		{"ImgName", "python_Main-p-u-g-s-t.core", "python"},
		{"ImgName2", "dummyimg_Main-p-u-g-s-t.core", "dummyimg"},
		{"Exe", "dummyexe-p-u-g-s-t.core", "dummyexe"},
		{"Exe2", "true-p-u-g-s-t.core", "true"},
		{"ExePath", "/path/to/true-p-u-g-s-t.core", "true"},
		{"ImgDashes", "servicename-rel6-1-0_svcname-main-12193-1000-10-6-1549589515.core", "servicename-rel6-1-0"},
	}
	for _, item := range data {
		t.Run(item.tstName, func(t *testing.T) {
			tlog := testlog.NewTestLog(t, true, false)
			found := parseCoreName(item.coreName)
			tlog.Freeze()
			out := tlog.Buf.String()
			if found != item.exe {
				t.Errorf("got %s, want %s\n", found, item.exe)
			}
			if t.Failed() {
				t.Logf("log content:\n%s\n", out)
			}
		})
	}
}

//func findExeWithGdb(fname string) (exe string)
func TestFindExeWithGdb(t *testing.T) {
	tlog := testlog.NewTestLog(t, true, false)
	dumpFile, testExe := testhelper.CoreHelper(t)
	defer os.Remove(dumpFile)
	defer os.Remove(testExe)

	exe := findExeWithGdb(dumpFile)
	if !strings.HasSuffix(exe, testExe) {
		t.Errorf("want %s, got %s\n", testExe, exe)
	}
	tlog.Freeze()
	if t.Failed() {
		t.Logf("log content:\n%s\n", tlog.Buf.String())
	}
}

//func zipOutput(zw *zip.Writer, name string, cmd *exec.Cmd)
func TestZipOutput(t *testing.T) {
	fi, err := os.Stat("pkg_test.go")
	if err != nil {
		t.Error("test setup failed:", err)
	}
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	cat := exec.Command("cat", "pkg_test.go")
	sha := exec.Command("sha1sum", "pkg_test.go")
	zipOutput(zw, "testfile", cat)
	zipOutput(zw, "sha", sha)
	zw.Close()
	reader := bytes.NewReader(buf.Bytes())
	zr, err := zip.NewReader(reader, int64(reader.Len()))
	if err != nil {
		t.Error(err)
	}
	if len(zr.File) != 2 {
		t.Errorf("want 2 files, got %d", len(zr.File))
	}
	var haveSha, haveFile bool
	var zipsha, zipfile []byte
	var zipfi os.FileInfo
	for _, f := range zr.File {
		content, err := f.Open()
		if err != nil {
			t.Error(err)
		}
		switch f.Name {
		case "sha":
			if haveSha {
				t.Error("duplicate entry for sha")
			}
			haveSha = true
			zipsha, err = ioutil.ReadAll(content)
			if err != nil {
				t.Error(err)
			}
		case "testfile":
			if haveFile {
				t.Error("duplicate entry for file")
			}
			haveFile = true
			zipfile, err = ioutil.ReadAll(content)
			if err != nil {
				t.Error(err)
			}
			zipfi = f.FileInfo()
		default:
			t.Errorf("unexpected file %s in zip", f.Name)
		}
	}
	if !haveFile {
		t.Error("missing file")
	}
	if !haveSha {
		t.Error("missing sha")
	}
	orig, err := ioutil.ReadFile("pkg_test.go")
	if err != nil {
		t.Error(err)
	}
	if !bytes.Equal(zipfile, orig) {
		t.Error("content mismatch")
	}
	hasher := sha1.New()
	if _, err := hasher.Write(orig); err != nil {
		t.Error(err)
	}
	hash := fmt.Sprintf("%x", hasher.Sum(nil))
	zsha := strings.TrimSpace(strings.Split(string(zipsha), " ")[0])
	if zsha != hash {
		t.Errorf("checksum differs\nwant %s\n got %s", hash, zsha)
	}
	if t.Failed() {
		t.Logf("input file: %v", fi)
		t.Logf("output file: %v", zipfi)
	}
}

//func zippedBacktrace(exe, core string, verbose bool) (buf *bytes.Buffer, err error)
func TestZippedBacktrace(t *testing.T) {
	tlog := testlog.NewTestLog(t, true, false)
	core, exe := testhelper.CoreHelper(t)
	defer os.Remove(core)
	defer os.Remove(exe)
	buf, err := zippedBacktrace(exe, core, true)
	if err != nil {
		t.Error(err)
	}
	reader := bytes.NewReader(buf.Bytes())
	zr, err := zip.NewReader(reader, int64(reader.Len()))
	if err != nil {
		t.Error(err)
	}
	extras := 0
	for i, f := range zr.File {
		t.Logf("element %d: %s", i, f.Name)
		if strings.HasPrefix(f.Name, extraDir) {
			extras++
		}
	}
	want := 3
	total := len(zr.File)
	if total-extras != want {
		t.Errorf("expected %d, got %d (not counting %d extras)", want, total-extras, extras)
	}
	tlog.Freeze()
	if t.Failed() {
		t.Logf("logged output:\n%s\n", tlog.Buf.String())
		f, err := ioutil.TempFile("", "go-core-bt*.zip")
		if err != nil {
			t.Fatal(err)
		}
		_, err = f.Write(buf.Bytes())
		if err != nil {
			t.Fatal(err)
		}
		f.Close()
		t.Logf("wrote zip to %s", f.Name())
	}
}
