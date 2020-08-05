// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package history

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/purecloudlabs/gprovision/pkg/common/strs"
)

//func (rl *resultList)moveOrAddFront(item *imageResult)
func TestMoaf(t *testing.T) {
	results = append(results,
		&ImageResult{Image: "img1"},
		&ImageResult{Image: "img2"})
	ir := &ImageResult{Image: "img3"}

	results.moveOrAddFront(ir)
	dumpResults(t)
	if len(results) != 3 {
		t.Fatalf("bad len %d", len(results))
	}
	if results[0].Image != "img3" {
		t.Errorf("expected img3 at index 0, got %s", results[0].Image)
	}
	results.moveOrAddFront(results[2])
	dumpResults(t)
	if len(results) != 3 {
		t.Errorf("bad len %d", len(results))
	}
}

//func RecordBootState(imgName string, success bool, severity uint)
func TestRecordBootState(t *testing.T) {
	ti := time.Now()
	splat := strings.SplitAfter(strs.ImgPrefix(), ".")
	var imgName string
	for i := range splat {
		if i == len(splat)-1 {
			break
		}
		imgName += splat[i]
	}
	imgName += "myImg"
	imgName2 := imgName + "2"
	results = nil
	dir, err := ioutil.TempDir("", "goTestHist")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	SetRoot(dir)
	RecordBootState(imgName, true, 0, ti, "")
	RecordBootState(imgName, true, 0, ti, "")
	checkCounts(t, 1, 0, 2, 0, true)
	RecordBootState(imgName2, true, 0, ti, "")
	RecordBootState(imgName, false, 1, ti, "")
	checkCounts(t, 2, 0, 3, 1, true)
	checkCounts(t, 2, 1, 1, 0, true)
	RecordBootState(imgName, false, 4, ti, "")
	checkCounts(t, 2, 0, 4, 5, true)
	checkCounts(t, 2, 1, 1, 0, true)
	RecordBootState(imgName2, false, 6, ti, "")
	checkCounts(t, 2, 0, 2, 6, false)
	checkCounts(t, 2, 1, 4, 5, true)
	RecordBootState(imgName, false, 1, ti, "")
	checkCounts(t, 2, 0, 5, 6, false)
	checkCounts(t, 2, 1, 2, 6, false)
	if t.Failed() {
		dumpResults(t)
	}
}
func checkCounts(t *testing.T, num, idx int, att, fail uint, pass bool) {
	t.Helper()
	if len(results) != num {
		t.Errorf("expected %d results, got %d", num, len(results))
	}
	if results[idx].BootAttempts != att {
		t.Errorf("#%d: expected %d attempts, got %d", idx, att, results[idx].BootAttempts)
	}
	if results[idx].BootFailures != fail {
		t.Errorf("#%d: expected %d failures, got %d", idx, fail, results[idx].BootFailures)
	}
	got := Check(results[idx].Image)
	if got != pass {
		t.Errorf("Check(%s): got %t, expect %t", results[idx].Image, got, pass)
	}
}

func dumpResults(t *testing.T) {
	//t.Helper() // requires go >= v1.9
	for i := range results {
		t.Logf("results[%d] == %#v\n", i, results[i])
	}
}
