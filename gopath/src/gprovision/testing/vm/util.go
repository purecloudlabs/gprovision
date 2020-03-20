// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package vm

import (
	"bytes"
	"gprovision/pkg/common/rlog"
	gtst "gprovision/testing"
	"io/ioutil"
	"os"
	"os/exec"
	fp "path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/u-root/u-root/pkg/qemu"
	"github.com/u-root/u-root/pkg/uroot"
)

//Create qcow-formatted file at path, with given size in bytes.
func CreateQcow(path string, siz uint64) error {
	qimg := os.Getenv("QEMU_IMG")
	if qimg == "" {
		qimg = "qemu-img"
	}
	return exec.Command(
		qimg,
		"create",
		"-f",
		"qcow2",
		path,
		strconv.FormatUint(siz, 10),
	).Run()
}

func DisableLDD(u *uroot.Opts) error {
	u.SkipLDD = true
	return nil
}

//for each fragment, runs q.ExpectRE() and errors if not found
func RequireTxt(t gtst.TB, q *qemu.VM, fragments ...string) {
	t.Helper()
	for _, f := range fragments {
		if _, err := q.ExpectRE(regexp.MustCompile(f)); err != nil {
			t.Errorf("error '%s' while waiting for\n%s", err, f)
		}
	}
}

//returns content of a file written by an integ test via 9p
//typically used in call(s) to RequireTxt9p()
func Find9p(t gtst.TB, tmpdir, glob string) []byte {
	matches, err := fp.Glob(fp.Join(tmpdir, "log", glob))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("want 1 match, got %d\n%#v", len(matches), matches)
	}
	data, err := ioutil.ReadFile(matches[0])
	if err != nil {
		t.Fatal(err)
	}
	return data
}

//look for items in a []byte, typically file content found by Find9p
func RequireTxt9p(t gtst.TB, p9log []byte, items ...string) {
	for _, item := range items {
		idx := bytes.Index(p9log, []byte(item))
		if idx < 0 {
			t.Errorf("cannot find %s in log", item)
			if strings.HasPrefix(item, "key:") {
				idx = bytes.Index(p9log, []byte("key:"))
				if idx >= 0 && len(p9log) > idx+30 {
					t.Logf("try %q", p9log[idx:idx+30])
				}
			}
		}
		itemEnd := idx + len(item)
		if len(p9log) > itemEnd {
			p9log = p9log[itemEnd:]
		}
	}
}

func SerNum(uefi bool) string {
	if uefi {
		return "QEMU01234U"
	}
	return "QEMU01234"
}

func Rawpath(tmpdir string, uefi bool) string {
	return fp.Join(tmpdir, "logserver", "raw", SerNum(uefi)+".raw")
}

//Look for indications of extra or wrong formatting verbs or args to printf
//and friends
//
// %!d(string=there)
// %!(EXTRA <nil>)
// %!s(MISSING)
// etc
func CheckFormattingErrs(t gtst.TB, lsrv rlog.MockSrvr, uefi bool) {
	//we pass TB rather than testing.T because it's an interface, so can be mocked

	content := []byte(lsrv.Entries(SerNum(uefi)))
	if len(content) == 0 {
		t.Fatal("no logs")
	}

	//Go's regexp is re2 compatible and defaults to "multi-line mode" off.
	//(?m) turns multi-line on, so that ^$ match beginning and end of line.
	//Each re here matches entire line to give some context.
	exprs := []string{
		`(?m)^.*%!.\(MISSING\).*$`, //extra directive
		`(?m)^.*%!\(EXTRA .*\).*$`, //extra arg
		`(?m)^.*%!.\(.*=.*\).*$`,   //wrong directive
	}
	for _, e := range exprs {
		re := regexp.MustCompile(e)
		matches := re.FindAll(content, -1)
		if len(matches) > 0 {
			t.Errorf("probable fmt mistakes for regex %q, matches:", e)
			for _, m := range matches {
				t.Logf("%s", string(m))
			}
		}
	}
}

func CheckForbidden(t gtst.TB, lsrv rlog.MockSrvr, uefi bool, strs []string) {
	content := lsrv.Entries(SerNum(uefi))
	if len(content) == 0 {
		t.Fatal("no logs")
	}
	for _, s := range strs {
		if strings.Contains(content, s) {
			t.Errorf("found forbidden string %s", s)
		}
	}
}
