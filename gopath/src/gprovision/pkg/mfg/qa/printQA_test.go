// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package qa

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"gprovision/pkg/appliance"
	"gprovision/pkg/common/rkeep"
	"gprovision/pkg/common/rlog"
	"gprovision/pkg/common/strs"
	futil "gprovision/pkg/fileutil"
	"gprovision/pkg/log"
	"gprovision/pkg/log/testlog"
	"gprovision/pkg/net/xfer"
	"io"
	"io/ioutil"
	"os"
	fp "path/filepath"
	"strings"
	"testing"
	"time"
)

func init() {
	log.SetPrefix("test")
}

//func QASummary(img Img, detected Specs, plat appliance.Variant) (d qavData)
func TestQASummary(t *testing.T) {
	if !rkeep.HaveRKeeper() || !rlog.HaveRLMock() || !rlog.HaveRLogSetup() {
		t.Skip("requires unavailable functionality")
	}
	img := dummyTV(t)
	s := &Specs{}
	p := &appliance.Variant{}
	t.Run("valid", func(t *testing.T) {
		tmp, err := ioutil.TempDir("", "gotest-qa")
		if err != nil {
			t.Error(err)
		}
		defer func() {
			if !t.Failed() {
				os.RemoveAll(tmp)
			} else {
				t.Logf("leaving temp dir %s", tmp)
			}
		}()
		tlog := testlog.NewTestLog(t, true, false)
		lSrv := rlog.MockServer(t, tmp)
		defer lSrv.Close()
		log.SetPrefix("go_test")
		err = rlog.Setup(fmt.Sprintf("http://localhost:%d", lSrv.Port()), "go_test")
		if err != nil {
			t.Error(err)
		}
		d := QASummary(img, s, p, nil)
		tlog.Freeze()
		if !d.Pass {
			t.Errorf("failed with valid checksum")
		}
		if t.Failed() {
			t.Log(tlog.Buf.String())
		}
	})
	t.Run("invalid", func(t *testing.T) {
		tmp, err := ioutil.TempDir("", "gotest-qa")
		if err != nil {
			t.Error(err)
		}
		defer func() {
			if !t.Failed() {
				os.RemoveAll(tmp)
			} else {
				t.Logf("leaving temp dir %s", tmp)
			}
		}()
		tlog := testlog.NewTestLog(t, true, false)
		lSrv := rlog.MockServer(t, tmp)
		defer lSrv.Close()
		log.SetPrefix("go_test")
		err = rlog.Setup(fmt.Sprintf("http://localhost:%d", lSrv.Port()), "go_test")
		if err != nil {
			t.Error(err)
		}
		img.Sha1 += "x"
		tlog.FatalIsNotErr = true
		d := QASummary(img, s, p, nil)
		tlog.Freeze()
		if tlog.FatalCount != 1 {
			t.Error("should call log.Fatal once")
		}
		if d.Pass {
			t.Errorf("passed with bad checksum")
		}
		if t.Failed() {
			t.Log(tlog.Buf.String())
		}
	})
}

//create a TVFile struct, backed by small file with valid checksum
func dummyTV(t *testing.T) *xfer.TVFile {
	f, err := ioutil.TempFile("", "go-test-qa")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	//fill with 10k random data
	urnd, err := os.Open("/dev/urandom")
	if err != nil {
		t.Fatal(err)
	}
	defer urnd.Close()
	l := io.LimitReader(urnd, 10240)
	_, err = io.Copy(f, l)
	if err != nil {
		t.Fatal(err)
	}
	//reset position, create hash
	_, err = f.Seek(0, io.SeekStart)
	if err != nil {
		t.Fatal(err)
	}
	hasher := sha1.New()
	_, err = io.Copy(hasher, f)
	if err != nil {
		t.Fatal(err)
	}
	sha := hasher.Sum(nil)
	shaString := fmt.Sprintf("%x", sha)
	img := &xfer.TVFile{Dest: f.Name(), Sha1: shaString}
	return img
}

//func (d qavData) Hardcopy()
func TestHardcopy(t *testing.T) {
	if !rkeep.HaveRKeeper() || !rlog.HaveRLMock() || !rlog.HaveRLogSetup() {
		t.Skip("requires unavailable functionality")
	}
	t.Run("invalid", func(t *testing.T) {
		tmp, err := ioutil.TempDir("", "gotest-qa")
		if err != nil {
			t.Error(err)
		}
		defer func() {
			if !t.Failed() {
				os.RemoveAll(tmp)
			} else {
				t.Logf("leaving temp dir %s", tmp)
			}
		}()
		tlog := testlog.NewTestLog(t, true, false)
		lSrv := rlog.MockServer(t, tmp)
		defer func() {
			lSrv.Close()
		}()
		log.SetPrefix("go_test")
		err = rlog.Setup(fmt.Sprintf("http://localhost:%d", lSrv.Port()), "go_test")
		if err != nil {
			t.Error(err)
		}
		d := qavData{}
		log.Msg("test with invalid data")
		buf := d.hardcopy()
		tlog.Freeze()
		l := tlog.Buf.String()
		if !strings.Contains(l, "not printing report") {
			t.Errorf("should not print\nlog=%s", l)
		}
		if buf.Len() > 0 {
			t.Errorf("expected len 0, got %d\nlog=%s\nbuf=%s", buf.Len(), l, buf.Bytes())
		}
	})
	//data shared by next 2
	d := qavData{}
	d.Pass = true
	d.Cpus.Cores = 9
	d.Cpus.Sockets = 8
	d.Cpus.Model = "fast"
	d.Img = &xfer.TVFile{
		Src:  "http://dasfsafs/PRODUCT.Os.Plat.imgimgimg.upd",
		Sha1: "shashashashashashashashashashashashashashashashashasha",
	}
	d.Model = "fancy"
	d.NumPci = 7
	d.NumUsb = 6
	d.SN = "007"
	d.CfgSteps = []string{"Apply BIOS settings with conrep"}

	t.Run("valid", func(t *testing.T) {
		tmp, err := ioutil.TempDir("", "gotest-qa")
		if err != nil {
			t.Error(err)
		}
		defer func() {
			if !t.Failed() {
				os.RemoveAll(tmp)
			} else {
				t.Logf("leaving temp dir %s", tmp)
			}
		}()
		tlog := testlog.NewTestLog(t, true, false)
		lSrv := rlog.MockServer(t, tmp)
		defer lSrv.Close()
		log.SetPrefix("go_test")
		err = rlog.Setup(fmt.Sprintf("http://localhost:%d", lSrv.Port()), "go_test")
		if err != nil {
			t.Fatal(err)
		}

		buf := d.hardcopy()
		tlog.Freeze()
		l := tlog.Buf.String()
		if buf.Len() < 2000 {
			t.Errorf("template error:\nlog=%s\nout=%s", l, buf.Bytes())
		}
		if bytes.Contains(buf.Bytes(), []byte("shashashashashashashashasha")) {
			t.Errorf("hash not truncated")
		}
		if bytes.Contains(buf.Bytes(), []byte(d.Img.Src)) {
			t.Errorf("Img.Src not trimmed")
		}
		if !bytes.Contains(buf.Bytes(), []byte("conrep")) {
			t.Errorf("ConfigSteps not listed")
		}
		t.Logf("%s", buf.Bytes())
	})
	t.Run("eeprom", func(t *testing.T) {
		tmp, err := ioutil.TempDir("", "gotest-qa")
		if err != nil {
			t.Error(err)
		} else {
			defer func() {
				if !t.Failed() {
					os.RemoveAll(tmp)
				} else {
					t.Logf("leaving temp dir %s", tmp)
				}
			}()
		}
		lSrv := rlog.MockServer(t, tmp)
		defer lSrv.Close()

		d.NicEepromFlash = true
		out := fp.Join(tmp, "logserver", "007_qav.htm")
		err = rlog.Setup(fmt.Sprintf("http://localhost:%d", lSrv.Port()), "go_test")
		if err != nil {
			t.Fatal(err)
		}
		pfx := strs.MfgKernel()
		pfx = strings.TrimSuffix(pfx, fp.Ext(pfx))
		log.SetPrefix(pfx) //used by ReportFinished to determine which stage finished

		// send file and notify that mfg finished
		d.Hardcopy()
		rkeep.ReportFinished("test mfg finished")

		time.Sleep(time.Second / 10)
		_, err = os.Stat(out)
		if err == nil {
			// file should not appear after mfg finishes, it should only appear when
			// fr finishes
			t.Errorf("%s exists before factory restore finished", out)
		}
		//notify that fr finished. file should be created shortly...
		log.SetPrefix("recov")
		rkeep.ReportFinished("test fr finished")

		futil.WaitFor(out, 5*time.Second)
		_, err = os.Stat(out)
		if err != nil {
			t.Errorf("factory restore finished but %s does not exist", out)
		} else {
			t.Logf("%s created - success", out)
		}
	})
}
