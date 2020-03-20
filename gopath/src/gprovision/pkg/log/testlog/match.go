// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

// +build !release

package testlog

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	fp "path/filepath"
	"strings"
)

//Filters log buffer, comparing remaining lines to want. Buffer is left empty.
//Assumes each entry is a single line.
func (tlog *TstLog) LinesMustMatch(lf LineFilterer, want []string) {
	tlog.t.Helper()
	tlog.LinesMustMatchCleaned(lf, nil, want)
}

//Like LinesMustMatch, but alters log lines before comparing to expected input
func (tlog *TstLog) LinesMustMatchCleaned(filterFn LineFilterer, cleanFn LineCleaner, want []string) bool {
	tlog.t.Helper()
	success, _ := tlog.linesMustMatchCleaned(filterFn, cleanFn, want)
	return success
}
func (tlog *TstLog) linesMustMatchCleaned(filterFn LineFilterer, cleanFn LineCleaner, want []string) (success bool, got []string) {
	tlog.Freeze()
	tlog.t.Helper()
	success = true
	if tlog.Buf == nil {
		tlog.t.Error("nil buffer")
		success = false
		return
	}
	b := tlog.Buf.Bytes()
	filtered := tlog.Filter(filterFn)
	tlog.t.Helper()
	if len(filtered) != len(want) {
		tlog.t.Errorf("len mismatch - got %d want %d", len(filtered), len(want))
		success = false
	}
	for i, l := range filtered {
		trimmed := l
		if cleanFn != nil {
			trimmed = cleanFn(l)
		}
		got = append(got, trimmed)
		if i < len(want) && trimmed != want[i] {
			tlog.t.Errorf("\n got %s\nwant %s\nraw[%d]=%s", trimmed, want[i], i, l)
			success = false
		}
	}
	if !success {
		if cleanFn != nil && len(filtered) > 0 {
			cleaned := []string{}
			for _, g := range filtered {
				cleaned = append(cleaned, cleanFn(g))
			}
			tlog.t.Logf("filtered/cleaned:\n%#v", cleaned)
		}
		tlog.t.Logf("wanted:\n%#v", want)
		if *DumpFull {
			if len(b) == 0 {
				tlog.t.Logf("all: <no entries>")
			} else {
				tlog.t.Logf("all:\n%s", string(b))
			}
		}
	}
	return
}

// UpdateGolden writes matching golden files that need updated. Test(s) still fail; re-run to verify.
//
// Example:
//   go test gprovision/pkg/hw/cfa -run PressAnyKey2 -updateGolden testdata/TestPressAnyKey2.golden
// This will update the correct file regardless of current working dir.
var UpdateGolden = flag.String("updateGolden", "", "during testing, allow updating this golden file")

// DumpFull writes the complete log to stderr if the test fails.
//
// Example:
//   go test myPkg -run myTest -dumpFull
var DumpFull = flag.Bool("dumpFull", false, "on failure, write out complete log")

// Like LinesMustMatch, but compares against a file instead. Easier to manage
// when the output volume is higher.
//
//    tlog := testlog.NewTestLog(t, true, false)
//    // do things that write to the log in a deterministic manner
//    log.Logf("step 1: write 0x%x to %s", val, path)
//    log.Logf("step 2: read %s from %s", val2, path2)
//    //...
//    // prevent any more writing to the log and flush channel
//    tlog.Freeze()
//    // now compare the filtered and/or trimmed output with known good output
//    // reads testdata/testName.golden or testdata/testName/subTestName.golden
//    // Use -updateGolden to create/update files; dir(s) must exist
//    tlog.MustMatchGoldenCleaned(testlog.FilterLogPfx("step"), nil)
func (tlog *TstLog) MustMatchGoldenCleaned(filterFn LineFilterer, cleanFn LineCleaner) bool {
	tlog.t.Helper()
	fname := fp.Join("testdata", tlog.t.Name()+".golden")
	var updateGolden, goldenWildcard bool
	if UpdateGolden != nil {
		goldenWildcard = strings.ContainsAny(*UpdateGolden, "*?")
		match, err := fp.Match(*UpdateGolden, fname)
		if err != nil {
			tlog.t.Fatalf("glob error: %s", err)
		}
		updateGolden = match
	}
	content, _ := ioutil.ReadFile(fname)
	want := strings.Split(string(content), "\n")
	last := len(want) - 1

	//remove up to 1 empty line at eof
	if want[last] == "" {
		want = want[:last]
	}
	success, got := tlog.linesMustMatchCleaned(filterFn, cleanFn, want)
	if updateGolden {
		if success && !goldenWildcard {
			//user should not use -updateGolden unless file needs to change
			//if file doesn't change, treat that as an error
			//exception: provided string contains wildcard(s)
			tlog.t.Fatal("no change to golden file")
		}
		if len(got) > 0 {
			f, err := os.Create(fname)
			if err != nil {
				tlog.t.Fatal(err)
			}
			defer f.Close()
			for _, line := range got {
				fmt.Fprintln(f, line)
			}
		} else {
			//no content
			os.Remove(fname)
		}
	}
	return success
}
