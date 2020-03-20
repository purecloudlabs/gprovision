// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

//Package vm contains utility functions used in integ tests with qemu VMs.
package vm

import (
	"archive/tar"
	"bytes"
	"crypto/sha1"
	"fmt"
	"gprovision/pkg/common/rlog"
	"gprovision/pkg/common/strs"
	pbsrvr "gprovision/pkg/oss/pblog/server"
	gtst "gprovision/testing"
	"gprovision/testing/fakeupd"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	fp "path/filepath"
	"strings"
	"testing"
	"text/template"
	"time"

	"github.com/u-root/u-root/pkg/qemu"
	"github.com/u-root/u-root/pkg/vmtest"
)

//waits for vm to exit or for the given time to elapse
func Wait(t gtst.TB, vm *qemu.VM, dly time.Duration) {
	done := make(chan struct{})
	var err error
	go func() {
		err = vm.Wait()
		close(done)
	}()
	select {
	case <-time.After(dly):
		t.Errorf("vm did not exit within timeout")
	case <-done:
		if err != nil {
			t.Error(err)
		}
	}
	vm.Close()

	if t.Failed() {
		fmt.Fprintf(os.Stderr, "\x1b[?7h")
		t.Logf("vm args: %s", vm.CmdlineQuoted())
	}
}

//default remotelog addr format, for pblog
var LogAddrFmt string = "10.0.2.2:%d"

//override to do any additional credential endpoint setup if not using pblog
var setCredEP = func(*Mockinfra) {}

//set up log server, webserver; return struct for mfg json url, log port, and cleanup func.
func MockInfra(tb gtst.TB, tmpDir, krnl string, createUpd bool, updPath, sn string, mem, cpus int) *Mockinfra {
	if tb == nil {
		mtb := &gtst.MockTB{}
		mtb.Underlying(&gtst.TBLogAdapter{})
		tb = mtb
	}
	if !rlog.HaveRLMock() {
		pbsrvr.UseMockImpl()
	}
	lSrvr := rlog.MockServer(tb, tmpDir)

	fListener, err := net.Listen("tcp", ":0")
	if err != nil {
		tb.Fatal(err)
	}

	// Some overhead in memory. not sure of an exact formula.
	// A memory figure is needed in manufData.json as part of the unit specs.
	switch mem {
	case 0, 512:
		mem = 475
	case 5120:
		mem = 4948
	default:
		tb.Logf("warning, memory value %s untested. assuming 5%% overhead.", mem)
		tb.Logf("if 5%% is too far off, mfg app will stop with an error")
		mem = int(0.95 * float32(mem))
	}

	infra := &Mockinfra{
		LSrvr:  lSrvr,
		t:      tb,
		sn:     sn,
		tmpDir: tmpDir,
		TmplData: TmplData{
			Mem:     mem,
			CPUs:    cpus,
			LAddr:   fmt.Sprintf(LogAddrFmt, lSrvr.Port()),
			FPort:   fListener.Addr().(*net.TCPAddr).Port,
			KName:   strs.BootKernel(),
			UpdName: strs.ImgPrefix() + "2006-01-02.1234.upd",
		},
		BootKernelPath: krnl,
	}
	if createUpd {
		tb.Logf("creating fake upd...")
		upd, err := fakeupd.Make(tmpDir, krnl, "")
		if err != nil {
			tb.Fatal(err)
		}
		if len(upd) < 1024*1024*15 {
			tb.Fatalf("undersize update %d", len(upd))
		}
		infra.updXz = upd
	} else {
		if len(updPath) > 0 {
			infra.useExistingUpd(updPath)
		}
	}

	// Originally we only set usingPb if rlog.HaveRLMock() returned false.
	// However this doesn't always work as the rlog mock may be initialized
	// by a previous test. Instead, check the type of lSrvr.
	_, infra.usingPb = lSrvr.(*pbsrvr.MockSrvr)
	if infra.usingPb {
		infra.TmplData.CredEP = "pblog"
	} else {
		setCredEP(infra)
	}
	//all TmplData must be set before calling Setup()
	infra.Setup()
	infra.fSrvr = &http.Server{
		Handler: infra,
	}
	go func() {
		err := infra.fSrvr.Serve(fListener)
		if err != nil && err != http.ErrServerClosed {
			tb.Error(err)
		}
	}()
	infra.MfgUrl = fmt.Sprintf("http://10.0.2.2:%d/manufData.json", infra.TmplData.FPort)
	return infra
}

//data used to populate manufData json and to serve other files used by mfg
type Mockinfra struct {
	t                      gtst.TB
	tmpDir, sn             string
	usingPb                bool
	LSrvr                  rlog.MockSrvr
	fSrvr                  *http.Server
	MfgUrl                 string
	BootKernelPath         string
	stashXz, updXz, kernel []byte
	updPath                string

	TmplData  TmplData
	manufData []byte //template output, json
}

//data that is available to template
type TmplData struct {
	Mem, CPUs              int
	FPort                  int
	CredEP                 string
	LAddr                  string
	KName, StashName       string
	CmdSum                 string
	StashSum, UpdSum, KSum string
	UpdName                string
}

func (mi *Mockinfra) Cleanup() {
	mi.t.Log("Mockinfra.Cleanup()")
	if mi.t.Failed() && mi.usingPb {
		lp := fp.Join(mi.tmpDir, "log.txt")
		entries := []byte(mi.LSrvr.Entries(mi.sn))
		err := ioutil.WriteFile(lp, entries, 0755)
		if err != nil {
			mi.t.Errorf("writing out log: %s", err)
		}
	}
	mi.LSrvr.Close()
	mi.fSrvr.Close()
}
func (mi *Mockinfra) useExistingUpd(upd string) {
	mi.updPath = upd
	mi.TmplData.UpdName = fp.Base(upd) //ends up in manufData template, and from there in disktag

	//calculate sha
	sha := sha1.New()
	f, err := os.Open(upd)
	if err != nil {
		mi.t.Fatal(err)
	}
	defer f.Close()
	_, err = io.Copy(sha, f)
	if err != nil {
		mi.t.Fatal(err)
	}
	mi.TmplData.UpdSum = fmt.Sprintf("%x", sha.Sum(nil))
}
func (mi *Mockinfra) LogUrl() string { return mi.TmplData.LAddr }

const (
	updDir = "/linux_mfg/Image/"
)

//must implement http.Handler interface
var _ http.Handler = (*Mockinfra)(nil)

// Override to add additional endpoints. Returns true if handled, false otherwise.
var otherEndpoints = func(m *Mockinfra, w http.ResponseWriter, ep string) bool { return false }

func (m *Mockinfra) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	url := r.URL.Path
	if !strings.HasPrefix(url, "/") {
		url = "/" + url
	}
	// update path in json changes so the disktag will match the actual image,
	// if a real image is used
	updUrl := updDir + m.TmplData.UpdName

	switch url {
	case "/manufData.json":
		_, err := w.Write(m.manufData)
		if err != nil {
			m.t.Fatal(err)
		}
		return
	case "/" + strs.BootKernel():
		n, err := w.Write(m.kernel)
		if err != nil {
			m.t.Error(err)
		} else if n != len(m.kernel) {
			m.t.Error(io.ErrShortWrite, n, len(m.kernel))
		}
		return
	case "/linux_mfg/stash.txz":
		//tarball with simple executable we can Expect on
		_, err := w.Write(m.stashXz)
		if err != nil {
			m.t.Fatal(err)
		}
		return
	case updUrl:
		if len(m.updPath) > 0 {
			f, err := os.Open(m.updPath)
			if err != nil {
				m.t.Fatal(err)
			}
			defer f.Close()
			_, err = io.Copy(w, f)
			if err != nil {
				m.t.Fatal(err)
			}
			return
		}
		_, err := w.Write(m.updXz)
		if err != nil {
			m.t.Fatal(err)
		}
		return
	case "/sampleCmd.sh":
		_, err := w.Write([]byte(sampleCmd_sh))
		if err != nil {
			m.t.Fatal(err)
		}
		return
	default:
		if otherEndpoints(m, w, url) {
			//handled
			return
		}
		w.WriteHeader(404)
		fmt.Fprintf(w, "unknown file %s\n", url)
		m.t.Fatalf("http dir server: unknown file %s requested", url)
	}
}

func (m *Mockinfra) Setup() {
	if len(m.updXz) == 0 && len(m.updPath) == 0 {
		m.t.Logf("creating upd from random data")
		//get random data to put in image tarball
		rnd, err := os.Open("/dev/urandom")
		if err != nil {
			m.t.Fatal(err)
			return
		}
		rndBuf, err := ioutil.ReadAll(io.LimitReader(rnd, 10*1024*1024))
		rnd.Close()
		if err != nil {
			m.t.Fatal(err)
			return
		}
		//create tar.xz's in memory
		m.updXz = tarXzBuf("random_file.bin", rndBuf, m.t, false, true)
	}
	m.stashXz = tarXzBuf(stash_sh_name, []byte(stash_sh), m.t, true, false)
	//checksums
	m.TmplData.CmdSum = fmt.Sprintf("%x", sha1.Sum([]byte(sampleCmd_sh)))
	m.TmplData.StashSum = fmt.Sprintf("%x", sha1.Sum(m.stashXz))
	if len(m.updXz) != 0 {
		m.TmplData.UpdSum = fmt.Sprintf("%x", sha1.Sum(m.updXz))
	}
	if len(m.BootKernelPath) > 0 {
		var err error
		m.kernel, err = ioutil.ReadFile(m.BootKernelPath)
		if err != nil {
			m.t.Error(err)
		}
		m.TmplData.KSum = fmt.Sprintf("%x", sha1.Sum(m.kernel))
	} else {
		//dummy file
		m.kernel = m.stashXz
		m.TmplData.KSum = m.TmplData.StashSum
	}
	m.prepareTmpl()
}

//Create manufData.json from template. Data source is m.TmplData.
func (m *Mockinfra) prepareTmpl() {
	tmpl := template.New("manufData")
	//override the default delims because of embedded templates we don't want to touch
	tmpl.Delims("[[", "]]")
	_, err := tmpl.Parse(mfgTmpl)
	if err != nil {
		m.t.Error(err)
		return
	}
	tbuf := &bytes.Buffer{}
	err = tmpl.Execute(tbuf, m.TmplData)
	if err != nil {
		fmt.Fprintf(tbuf, "templating manufData.json: %s\n", err)
		m.t.Error(err)
	}
	m.manufData = tbuf.Bytes()
}

//create tar.xz in-memory with a single file. optional: executable, xz sha256 checksum
func tarXzBuf(name string, content []byte, t gtst.TB, exe, sha bool) []byte {
	tbuf, xbuf := &bytes.Buffer{}, &bytes.Buffer{}
	var mode int64 = 0644
	if exe {
		mode = 0755
	}
	tr := tar.NewWriter(tbuf)
	err := tr.WriteHeader(&tar.Header{
		Name:    name,
		Size:    int64(len(content)),
		ModTime: time.Now(),
		Mode:    mode,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = tr.Write(content)
	if err != nil {
		t.Fatal(err)
	}
	tr.Close()
	//sha256 checksum is required for image; does not hurt for others
	xz := exec.Command("xz")
	if sha {
		xz.Args = append(xz.Args, "-C", "sha256")
	}
	xz.Stdin = tbuf
	xz.Stdout = xbuf
	err = xz.Run()
	if err != nil {
		t.Fatal(err)
	}
	return xbuf.Bytes()
}

// WARNING the data within this template can itself be templated, so the
// template chars we use are [[ and ]] rather than the default of {{ and }}.
//
// Fields available for templating are in TmplData struct.
const mfgTmpl = `
{
  "_comment": "if ApplianceJsonUrl is specified, that url is used for identification and then mfg will halt",
  "_ApplianceJsonUrl": "http://10.0.2.2:[[ .FPort ]]/infra/appliance-qemu-ipmi.json",
  "Files": [
    {
      "_comment1": "if omitted, Dest can be determined for *.upd and anything going in the root dir of RECOVERY/",
      "_comment2": "the destination image directory must _always_ be capitalized",
      "_comment3": "adding an underscore has the same effect as omitting Dest entirely, except you can see what it _would_ look like",
      "_Dest": "Image/PRODUCT.Os.Plat.2017-08-15.6240.upd",
      "Src": "http://10.0.2.2:[[ .FPort ]]/linux_mfg/Image/[[ .UpdName ]]",
      "Sha1": "[[ .UpdSum ]]"
	},
	{
	  "Src": "http://10.0.2.2:[[ .FPort ]]/[[ .KName ]]",
	  "Sha1": "[[ .KSum ]]"
	}
  ],
  "LogEndpoint": "[[ .LAddr ]]",
  "StashFiles": [
    {
      "Src": "http://10.0.2.2:[[ .FPort ]]/linux_mfg/stash.txz",
      "Sha1": "[[ .StashSum ]]"
    }
  ],
  "CredentialEndpoint": "[[ .CredEP ]]",
  "ValidationData": [
    {
      "DevCodeName": "QEMU-mfg-test",
      "RamMegs": [[ .Mem ]],
      "Recovery": {
        "_comment": "10G",
        "Size": 10737418240,
        "SizeTolerancePct": 1,
        "Vendor": "QEMU",
        "Model": "QEMU HARDDISK"
      },
      "MainDiskConfigs": [
        [
          {
            "_comment": "30G x 2",
            "Size": 30000000000,
            "SizeTolerancePct": 1,
            "Vendor": "ATA",
            "Model": "root30g",
            "Quantity": 2
          }
        ],
        [
          {
            "_comment": "2G",
            "Size": 2147483648,
            "SizeTolerancePct": 1,
            "Vendor": "ATA",
            "Model": "root2g",
            "Quantity": 1
          }
        ],
        [
          {
            "_comment": "20G",
            "Size": 21474836480,
            "SizeTolerancePct": 1,
            "Vendor": "ATA",
            "Model": "roothdd",
            "Quantity": 1
          }
        ],
        [
          {
            "_comment": "200G",
            "Size": 214748364800,
            "SizeTolerancePct": 1,
            "Vendor": "ATA",
            "Model": "root200g",
            "Quantity": 1
          }
        ]
      ],
      "OUINicsSequential": true,
      "NumOUINics": 1,
      "TotalNics": 1,
      "CPUInfo": {
		"_comment": "determined by qemu -cpu arg",
        "Model": "QEMU Virtual CPU version 2.5+",
        "Cores": [[ .CPUs ]],
        "Sockets": [[ .CPUs ]]
      }
    }
  ],
  "CustomPlatCfgSteps": [
    {
      "DevCodeName": "QEMU-mfg-test",
      "ConfigSteps": [
        {
          "Name": "Example step",
          "When": "RunAfterImaging",
          "Verbose": true,
          "Files": [
            {
              "Src": "http://10.0.2.2:[[ .FPort ]]/sampleCmd.sh",
              "Sha1": "[[ .CmdSum ]]"
            }
          ],
          "Commands": [
            {
              "Command": "chmod +x {{.DLDir}}/sampleCmd.sh",
              "ExitStatus": "ESMustSucceed",
              "AddPath": "",
              "AddLibPath": ""
            },
            {
              "Command": "{{.DLDir}}/sampleCmd.sh"
            }
          ]
        },
        {
          "Name": "Another step",
          "When": "RunAfterPWSet",
          "Files": [],
		  "Verbose": true,
          "Commands": [
            {
              "Command": "ls -lR {{ .RecoveryDir }} "
            },
            {
              "Command": "echo serial={{.Serial}}"
            },
            {
              "Command": "echo {{ .OSPass }} {{ .BiosPass }} {{ .IpmiPass }}"
            }
          ]
        }
      ]
    }
  ]
}
`

const sampleCmd_sh = `#!/bin/sh
echo "sample command executing..."
sha1sum /init
echo "sample command done"
`

var stash_sh_name = "stash.sh"
var stash_sh = `#!/bin/sh
echo "stash command executing..."
`

type nopCloserW struct {
	io.Writer
}

func (n *nopCloserW) Close() error { return nil }

func NopCloserW(w io.Writer) io.WriteCloser {
	return &nopCloserW{w}
}

// Returns a buffer and a WriteCloser. Data written to wc is written to t and
// copied to buf. Must use testing.T because TestLineWriter requires that.
func CopyOutput(t *testing.T) (*bytes.Buffer, io.WriteCloser) {
	obuf := &bytes.Buffer{}
	multi := io.MultiWriter(
		vmtest.TestLineWriter(t, "serial"),
		obuf,
	)
	return obuf, &nopCloserW{multi}
}
