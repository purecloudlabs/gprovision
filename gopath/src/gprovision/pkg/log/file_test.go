// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package log_test

// Note that this is package log_test, not log. Ensures that we expose enough
// functions to make testing possible from other packages.

import (
	"gprovision/pkg/log"
	"gprovision/pkg/log/flags"
	"io/ioutil"
	"os"
	fp "path/filepath"
	"testing"
	"time"
)

func TestFileLog(t *testing.T) {
	log.DefaultLogStack()
	defer log.DefaultLogStack() //cleanup when test is done
	T, err := time.Parse("2006", "1999")
	if err != nil {
		t.Fatal(err)
	}
	e := log.LogEntry{
		Time:  T,
		Msg:   "interesting event",
		Flags: flags.EndUser,
	}
	stack := log.Stack()
	stack.AddEntry(e)
	entries := log.StoredEntries()
	if len(entries) != 1 {
		t.Error("wrong entries", entries)
	}
	//add another event, this time one that should not make it into the file
	e.Time = T.Add(time.Minute)
	e.Msg = "sensitive event"
	e.Flags = flags.EndUser | flags.NotFile
	stack.AddEntry(e)
	entries = log.StoredEntries()
	if len(entries) != 2 {
		t.Error("wrong entries", entries)
	}

	tmp, err := ioutil.TempDir("", "gotest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)
	log.SetPrefix("gotest")
	_, err = log.AddFileLog(tmp)
	if err != nil {
		t.Fatal(err)
	}
	log.Finalize()
	fn, success := log.GetAttr("Filename")
	if !success {
		t.Error("no Filename attr")
	}
	fi, err := os.Stat(fn.(string))
	if err != nil {
		t.Fatal(err)
	}
	want := "-- 19990101_0000 -- interesting event\n"
	if fi.Size() != int64(len(want)) {
		t.Errorf("want %d, got %d", len(want), fi.Size())
	}
	buf, err := ioutil.ReadFile(fp.Join(tmp, fi.Name()))
	if err != nil {
		t.Fatal(err)
	}
	if string(buf) != want {
		t.Errorf("file:\nwant %q\ngot  %q", want, string(buf))
	}
}
