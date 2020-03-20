// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package server

import (
	"gprovision/pkg/oss/pblog/pb"
	"io/ioutil"
	"os"
	fp "path/filepath"
	"testing"
	"time"
)

func TestHoldDoc(t *testing.T) {
	RawDir, err := ioutil.TempDir("", "go-test-lsrv")
	if err != nil {
		t.Error(err)
	}
	defer func() {
		err := os.RemoveAll(RawDir)
		if err != nil {
			t.Error(err)
		}
	}()

	PrintDir = fp.Join(RawDir, "print")
	err = os.Mkdir(PrintDir, 0777)
	if err != nil {
		t.Error(err)
	}
	a := &allInOneSrvr{store: newMockStore()}
	QAHold = time.Second
	done := make(chan struct{})
	defer close(done)
	a.MonitorHolds(done)
	d := PrintableDoc{
		Document: &pb.Document{
			Name: "testdoc",
			Body: []byte("testdoc"),
			SN:   "testsn",
		},
		Expires: time.Now().Add(-1 * time.Hour),
	}
	HoldForPrinting <- d
	time.Sleep(2 * time.Second)
	ReleaseForPrinting <- "testsn"
	entries, err := ioutil.ReadDir(PrintDir)
	if err != nil {
		t.Error(err)
	}
	if len(entries) != 0 {
		t.Errorf("doc should have expired but was printed\n%v", entries)
	}
	HoldForPrinting <- d
	time.Sleep(time.Second / 2)
	ReleaseForPrinting <- "testsn"
	time.Sleep(time.Second / 2)
	entries, err = ioutil.ReadDir(PrintDir)
	if err != nil {
		t.Error(err)
	}
	if len(entries) != 1 {
		t.Errorf("doc should have printed but didn't\n%v", entries)
	}
}
