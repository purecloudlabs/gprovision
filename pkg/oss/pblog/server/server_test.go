// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package server

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/purecloudlabs/gprovision/pkg/common"
	"github.com/purecloudlabs/gprovision/pkg/common/rkeep"
	"github.com/purecloudlabs/gprovision/pkg/common/rlog"
	"github.com/purecloudlabs/gprovision/pkg/common/strs"
	"github.com/purecloudlabs/gprovision/pkg/log"
	"github.com/purecloudlabs/gprovision/pkg/log/testlog"
	"github.com/purecloudlabs/gprovision/pkg/oss/pblog"
)

func TestServer(t *testing.T) {
	tlog := testlog.NewTestLog(t, true, false)
	tmpDir, err := ioutil.TempDir("", "test-gprov-pbsrv")
	if err != nil {
		t.Error(err)
	}
	defer func() { os.RemoveAll(tmpDir) }()

	UseMockImpl()

	lSrvr := rlog.MockServer(t, tmpDir)
	t.Logf("listening at %d", lSrvr.Port())
	host := fmt.Sprintf("localhost:%d", lSrvr.Port())
	pblog.UseRLoggerSetup()
	pblog.UseRKeeper()
	log.SetPrefix(strs.MfgLogPfx())
	err = rlog.Setup(host, "testsn")
	if err != nil {
		t.Fatal(err)
	}

	rkeep.SetUnit(common.Unit{Platform: &common.PlatMock{Ser: "testsn"}})

	//make sure http access works
	resp, err := http.Get("http://" + host + "/recent/")
	if err != nil {
		t.Error(err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("http status %s", resp.Status)
	}

	log.Log("test")
	tm := rkeep.GetTime()
	if len(tm) < 19 {
		t.Errorf("reported time too short: %d (%s)", len(tm), tm)
	}
	rkeep.ReportCodename("codename")

	//now check what the server stored
	msrv := lSrvr.(*MockSrvr)
	ms := msrv.store.(*mockStore)
	t.Logf("im=%d, m=%d, l=%d", len(ms.imacs), len(ms.macs), len(ms.logs))
	e := lSrvr.Entries("testsn")

	if strings.Count(e, "\n") < 4 {
		t.Error("undersize log")
		t.Logf("entries:\n%s", e)
	}
	e = lSrvr.Entries("")
	if len(e) != 0 {
		t.Errorf("entries with no sn:\n%s", e)
	}
	tlog.Freeze()
	if t.Failed() {
		t.Log(tlog.Buf.String())
	}
}
