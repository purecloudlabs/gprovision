// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package fileutil

import (
	"gprovision/pkg/log/testlog"
	"io/ioutil"
	"os"
	fp "path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

//func ListFilesAndSize(dir, pattern string) (size int, files []string)
func TestListFilesAndSize(t *testing.T) {
	tlog := testlog.NewTestLog(t, true, false)
	defer func() {
		tlog.Freeze()
		if t.Failed() {
			t.Logf("log: %s", tlog.Buf.String())
		}
	}()
	dir, err := fp.Abs("../fileutil")
	if err != nil {
		t.Fatal(err)
	}
	s, list := ListFilesAndSize(dir, "*.go")
	if len(list) < 5 {
		t.Errorf("expect at least 5 files, got %d", len(list))
	}
	t.Logf("Size: %f", float64(s)/oneM)
	t.Logf("Files: %d, %v", len(list), list)
	for _, f := range list {
		if f[0] != '/' {
			t.Errorf("want absolute path, got %s", f)
		}
		_, err := os.Stat(f)
		if err != nil {
			t.Errorf("err stat %s: %s", f, err)
		}
		if !strings.HasSuffix(f, ".go") {
			t.Errorf("glob mismatch - want *.go, got %s", f)
		}
	}
	s, list = ListFilesAndSize(dir, "*.exe")
	if s != 0 {
		t.Errorf("want 0B size, got %dB", s)
	}
	if len(list) != 0 {
		t.Errorf("want 0 files, got %d: %v", len(list), list)
	}
}

//func CopySomeFiles(srcDir, destDir string, flist []string) error
func TestCopySomeFiles(t *testing.T) {
	dir := "../fileutil/kver"
	_, list := ListFilesAndSize(dir, "*.go")
	if len(list) == 0 {
		t.Errorf("no items in list")
	}
	dest, err := ioutil.TempDir("", "gotest")
	if err != nil {
		t.Errorf("creating temp dir: %s", err)
	}
	err = CopySomeFiles(fp.Dir(dir), dest, list)
	if err != nil {
		t.Errorf("copying files: %s", err)
	}
	_, err = os.Stat(fp.Join(dest, "kver/kver.go"))
	if err != nil {
		t.Errorf("file should exist: %s", err)
	}
	if !t.Failed() {
		err = os.RemoveAll(dest)
		if err != nil {
			t.Errorf("rm temp dir: %s", err)
		}
	}
}

//func DirMatchCaseInsensitive(dir string, entry string) (result string) {
func TestDirMatchCaseInsensitive(t *testing.T) {
	type tds struct {
		actual, request string
		match           bool
	}
	tmp, err := ioutil.TempDir("", "")
	if err != nil {
		t.Error(err)
	}
	defer os.RemoveAll(tmp)
	td := []tds{
		{"loGdir", "LOGDIR", true},
		{"loGDI", "LOGDIR", false},
		{"LOGDI", "LOGDIR", false},
		{"LOGDIRS", "LOGDIR", false},
		{"LOGDIR", "LOGDIR", true},
	}
	for i, d := range td {
		t.Run(d.actual, func(t *testing.T) {
			actual := fp.Join(tmp, d.actual)
			if err := os.MkdirAll(actual, 0777); err != nil {
				t.Error(err)
			}
			hit, got := DirMatchCaseInsensitive(tmp, []string{d.request})
			os.RemoveAll(actual)
			if hit != d.match {
				t.Errorf("DirMatchCaseInsensitive falsely reports no matches")
			}
			if (len(got) == 1 && (got[0] == actual)) != d.match {
				t.Errorf("%d: got=%s, actual=%s, request=%s (expected match = %t)", i, got, d.actual, d.request, d.match)
			} else {
				t.Logf("%d: got=%s, actual=%s, request=%s, match=%t", i, got, d.actual, d.request, d.match)
			}
		})
	}
	names := []string{
		"ProdReboot.txt",
		"ProdFactoryRestore.txt",
		"PRODLOGS",
		"ProdNetDefaults.txt",
	}
	for _, f := range names {
		err := ioutil.WriteFile(fp.Join(tmp, f), []byte(f), 0644)
		if err != nil {
			t.Error(err)
		}
	}
	t.Run("many", func(t *testing.T) {
		hit, got := DirMatchCaseInsensitive(tmp, names)
		if !hit {
			t.Error("no hits")
		}
		if len(got) != len(names) {
			t.Errorf("expected %d, got %d: %v", len(names), len(got), got)
		}
		for i := range names {
			got := strings.ToLower(fp.Base(got[i]))
			if names[i] != got {
				t.Errorf("got %s, want %s", got, names[i])
			}
		}
	})
	t.Run("none", func(t *testing.T) {
		for i := range names {
			names[i] = "ProdZ" + names[i][5:]
		}
		hit, got := DirMatchCaseInsensitive(tmp, names)
		if hit {
			t.Error("hits")
		}
		if len(got) != len(names) {
			t.Errorf("expected %d, got %d: %v", len(names), len(got), got)
		}
		for _, g := range got {
			if len(g) > 0 {
				t.Errorf("expected empty string, got %s: %v", g, got)
			}
		}
	})
	for _, tst := range []struct {
		l string
		i int
	}{{"R", 0}, {"F", 1}, {"L", 2}, {"N", 3}} {
		t.Run(tst.l, func(t *testing.T) {
			for i := range names {
				names[i] = "Prod" + tst.l + names[i][5:]
			}
			hit, got := DirMatchCaseInsensitive(tmp, names)
			if !hit {
				t.Error("no hits")
			}
			if len(got) != len(names) {
				t.Errorf("expected %d, got %d: %v", len(names), len(got), got)
			}
			for i, g := range got {
				if i != tst.i {
					if len(g) > 0 {
						t.Errorf("expected empty string, got %s: %v", g, got)
					}
				} else {
					if strings.ToLower(fp.Base(g)) != names[i] {
						t.Errorf("expected %s, got %s", names[i], g)
					}
				}
			}
		})
	}
}

//func WaitFor(path string, timeout time.Duration) (found bool)
func TestWaitFor(t *testing.T) {
	name, err := ioutil.TempDir("", "goTestWaitFor")
	if err != nil {
		t.Error(err)
	}
	defer os.RemoveAll(name)
	fname := fp.Join(name, "testfile")
	runWait := func(w, c time.Duration) {
		t.Logf("waiting %s for %s", w.String(), fname)
		found := WaitFor(fname, w)
		expect := false
		if w > c {
			expect = true
		}
		if found != expect {
			t.Errorf("got %t, want %t with w=%s", found, expect, w.String())
		} else {
			t.Logf("as expected: found=%t with w=%s", found, w.String())
		}
	}
	creatDelay := 2 * time.Second
	go runWait(time.Second, creatDelay)
	go runWait(3*time.Second, creatDelay)
	time.Sleep(creatDelay)
	if err := ioutil.WriteFile(fname, nil, 0777); err != nil {
		t.Error(err)
	}
	time.Sleep(2 * time.Second)
}

//func CheckDirPeriodic(dir string, delay time.Duration, action func() error)
func TestCheckDir(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "go-test-corer")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)
	wDir := tmpdir + "/wdir"
	err = os.Mkdir(wDir, 0755)
	if err != nil {
		t.Fatal(err)
	}

	changed := false
	var wg sync.WaitGroup
	action := func() error {
		changed = true
		wg.Done()
		//any error causes checkDir to return, terminating the goroutine
		return os.ErrNoDeadline
	}
	wg.Add(1)
	go CheckDirPeriodic(wDir, time.Second/5, action)
	time.Sleep(time.Second / 10)
	if changed {
		t.Fatal("change already seen")
	}
	err = os.Rename(wDir, wDir+"2")
	if err != nil {
		t.Fatal(err)
	}
	err = os.Symlink(wDir+"2", wDir)
	if err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	if !changed {
		t.Fatal("change not seen")
	}
}

//func WaitForDir(watchedIsMountpoint bool, watchDir string)
func TestWaitForDir(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "go-test-fileutil")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)
	wDir := tmpdir + "/wdir"
	wDir2 := tmpdir + "/wdir2"
	err = os.Mkdir(wDir, 0755)
	if err != nil {
		t.Fatal(err)
	}

	seen := false
	var v, w sync.WaitGroup
	v.Add(1)
	w.Add(1)
	go func() {
		v.Done()
		WaitForDir(false, wDir2)
		seen = true
		w.Done()
	}()
	v.Wait()
	time.Sleep(time.Second)
	if seen {
		t.Error("seen")
	}
	err = os.Symlink(wDir, wDir2)
	if err != nil {
		t.Error(err)
	}
	w.Wait()
	if !seen {
		t.Error("not seen")
	}
}

//func mpFromLine(l string) string
func TestMpFromLine(t *testing.T) {
	testdata := []struct {
		line, want string
	}{
		{
			line: "19 25 0:4 / /proc rw,nosuid,nodev,noexec,relatime shared:13 - proc proc rw",
			want: "/proc",
		},
		{
			line: "17 25 0:4 / /proc rw,relatime - proc proc rw",
			want: "/proc",
		},
	}
	for _, td := range testdata {
		got := mpFromLine(td.line)
		if got != td.want {
			t.Errorf("%s: got %s want %s", td.line, got, td.want)
		}
	}
}

//func isMountpoint(dir string) bool
func TestMountpoint(t *testing.T) {
	testdata := []struct {
		path string
		want bool
	}{
		{"/proc", true},
		{"/sdf sa", false},
	}
	for _, td := range testdata {
		got := IsMountpoint(td.path)
		if got != td.want {
			t.Errorf("%s: got %t want %t", td.path, got, td.want)
		}
	}
}

//func FindCaseInsensitive(root, name string, maxdepth int) (files []string, err error)
func TestFindCaseInsensitive(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "go-test-futil-find")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if t.Failed() {
			t.Logf("not deleting temp dir %s", tmpdir)
		} else {
			if err = os.RemoveAll(tmpdir); err != nil {
				t.Fatal(err)
			}
		}
	}()
	//set up test files
	files := []string{"a/b", "a/c/d", "d/e", "f/d", "b"}
	for _, f := range files {
		p := fp.Join(tmpdir, f)
		err = os.MkdirAll(fp.Dir(p), 0777)
		if err != nil {
			t.Fatal(err)
		}
		err = ioutil.WriteFile(p, []byte(p), 0644)
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, td := range []struct {
		name     string
		search   string
		maxdepth int
		results  []string
	}{
		{
			name:     "a",
			search:   "a",
			maxdepth: 10,
			results:  []string{},
		},
		{
			name:     "b",
			search:   "b",
			maxdepth: 10,
			results:  []string{files[0], files[4]},
		},
		{
			name:     "d",
			search:   "d",
			maxdepth: 1,
			results:  []string{files[3]},
		},
		{
			name:     "d2",
			search:   "d",
			maxdepth: 2,
			results:  []string{files[1], files[3]},
		},
	} {
		t.Run(td.name, func(t *testing.T) {
			res, err := FindCaseInsensitive(tmpdir, td.search, td.maxdepth)
			if err != nil {
				t.Fatal(err)
			}
			if len(res) != len(td.results) {
				t.Errorf("different number of results - got %d want %d", len(res), len(td.results))
			}
			for i := range res {
				res[i] = strings.TrimPrefix(res[i], tmpdir+string(os.PathSeparator))
			}
			sort.Strings(res)
			sort.Strings(td.results)
			for i := range res {
				if len(td.results) > i && res[i] != td.results[i] {
					t.Errorf("mismatch at index %d", i)
				}
			}
			if t.Failed() {
				t.Logf("\nwant: %s\ngot: %s", td.results, res)
			}
		})
	}
}
