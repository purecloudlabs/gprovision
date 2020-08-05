// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package vm

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/purecloudlabs/gprovision/pkg/common"
	"github.com/purecloudlabs/gprovision/pkg/mfg/mdata"
	gtst "github.com/purecloudlabs/gprovision/testing"
)

//ensure the template is functional and that it produces valid json
func TestMfgDataJson(t *testing.T) {
	m := &Mockinfra{
		t: t,
		TmplData: TmplData{
			LAddr:    "http://10.0.2.2:65432/",
			FPort:    8901,
			StashSum: "mfgsum",
			UpdSum:   "updsum",
			CmdSum:   "cmdsum",
			CredEP:   "CredEndpt",
		},
	}
	m.prepareTmpl()
	var md mdata.MfgDataStruct
	err := json.Unmarshal(m.manufData, &md)
	if err != nil {
		t.Error(err)
		t.Logf("%s", m.manufData)
	}
	for _, str := range []string{
		m.TmplData.LAddr,
		m.TmplData.StashSum,
		m.TmplData.UpdSum,
		m.TmplData.CmdSum,
		m.TmplData.CredEP,
	} {
		if !strings.Contains(string(m.manufData), str) {
			t.Errorf("missing: %s", str)
		}
	}
}

//verify the mock files are valid
func TestXz(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "gotest")
	if err != nil {
		t.Fatal(err)
	}
	infra := MockInfra(t, tmpdir, "", false, "", "", 0, 1)
	defer func() {
		if t.Failed() {
			return
		}
		infra.Cleanup()
		os.RemoveAll(tmpdir)
	}()
	//the url's ip won't work outside a vm - fix
	url := strings.Replace(infra.MfgUrl, "10.0.2.2", "localhost", -1)
	url = strings.TrimSuffix(url, "manufData.json")

	for _, td := range []struct {
		name, path, file string
	}{
		{
			name: "upd",
			path: strings.TrimPrefix(updDir+infra.TmplData.UpdName, "/"),
			file: "random_file.bin",
		},
		{
			name: "iem",
			path: "linux_mfg/stash.txz",
			file: stash_sh_name,
		},
	} {
		resp, err := http.Get(url + td.path)
		if err != nil {
			t.Fatal(err)
		}
		tar := exec.Command("tar", "tJ")
		tar.Stdin = resp.Body
		out, err := tar.CombinedOutput()
		if err != nil {
			t.Error(err)
		}
		f := strings.TrimSpace(string(out))
		if f != td.file {
			t.Errorf("want %s\ngot  %s", td.file, f)
		}
	}
}

//mock log server used to test CheckFormattingErrors
type mlsrv struct {
	content string
}

func (m *mlsrv) Entries(_ string) string             { return m.content }
func (*mlsrv) CheckFinished(_, _ string) bool        { panic("unimplemented") }
func (*mlsrv) Close()                                { panic("unimplemented") }
func (*mlsrv) MockCreds(_ string) common.Credentials { panic("unimplemented") }
func (*mlsrv) Port() int                             { panic("unimplemented") }
func (*mlsrv) Ids() []string                         { panic("unimplemented") }

//func CheckFormattingErrs(t testing.TB, lsrv rlog.MockSrvr, uefi bool)
func TestCheckFormattingErrs(t *testing.T) {
	for _, td := range []struct {
		name, content string
		res           gtst.MockTB
	}{
		{
			name:    "1",
			content: "\n%!d(string=there)\n%!(EXTRA <nil>)\n%!s(MISSING)\n",
			res:     gtst.MockTB{ErrCnt: 3, LogCnt: 3},
		},
		{
			name:    "2",
			content: "\n%!d(string=there)\n",
			res:     gtst.MockTB{ErrCnt: 1, LogCnt: 1},
		},
		{
			name:    "3",
			content: "\n%!(EXTRA <nil>)\n",
			res:     gtst.MockTB{ErrCnt: 1, LogCnt: 1},
		},
		{
			name:    "4",
			content: "\n%!s(MISSING)\n",
			res:     gtst.MockTB{ErrCnt: 1, LogCnt: 1},
		},
		{
			name:    "5",
			content: "\n11/25/2019-12:04:05 PM [macs] -- 00:26:fd:00:26:fd%!(EXTRA <nil>)\n",
			res:     gtst.MockTB{ErrCnt: 1, LogCnt: 1},
		},
		{
			name:    "6",
			content: "%!s(MISSING)",
			res:     gtst.MockTB{ErrCnt: 1, LogCnt: 1},
		},
		{
			name:    "7",
			content: "blah %!s(MISSING) blah",
			res:     gtst.MockTB{ErrCnt: 1, LogCnt: 1},
		},
	} {
		t.Run(td.name, func(t *testing.T) {
			mls := &mlsrv{content: td.content}
			mt := &gtst.MockTB{}
			CheckFormattingErrs(mt, mls, false)
			if mt.LogCnt != td.res.LogCnt {
				t.Errorf("log: want %d, got %d", td.res.LogCnt, mt.LogCnt)
			}
			if mt.ErrCnt != td.res.ErrCnt {
				t.Errorf("err: want %d, got %d", td.res.ErrCnt, mt.ErrCnt)
			}
			if mt.FatalCnt != td.res.FatalCnt {
				t.Errorf("fatal: want %d, got %d", td.res.FatalCnt, mt.FatalCnt)
			}
			if mt.Skp != td.res.Skp {
				t.Errorf("skip: want %t, got %t", td.res.Skp, mt.Skp)
			}
		})
	}
}

func TestLogInfra(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "test-gprov-linfra")
	if err != nil {
		t.Error(err)
	}
	infra := MockInfra(t, tmpdir, "", false, "", SerNum(false), 0, 1)
	host := strings.Replace(infra.LogUrl(), "10.0.2.2", "localhost", -1)
	if !strings.HasPrefix(host, "http://") {
		host = "http://" + host
	}
	host = strings.TrimSuffix(host, "/")
	t.Logf("host %s", host)
	resp, err := http.Get(host + "/recent")
	if err != nil {
		t.Error(err)
	}
	t.Log(resp, resp.Body)
}
