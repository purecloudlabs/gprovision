// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package main

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/purecloudlabs/gprovision/pkg/corer/opts"
	"github.com/purecloudlabs/gprovision/pkg/corer/testhelper"
	futil "github.com/purecloudlabs/gprovision/pkg/fileutil"
	"github.com/purecloudlabs/gprovision/pkg/log"
	"github.com/purecloudlabs/gprovision/pkg/log/testlog"

	"github.com/rjeczalik/notify"
)

//func handleEvent(ei notify.EventInfo, cfg *config)
func TestHandleEvent(t *testing.T) {
	tlog := testlog.NewTestLog(t, true, false)
	dumpFile, testExe := testhelper.CoreHelper(t)
	defer os.Remove(dumpFile)
	defer os.Remove(testExe)

	var ei notify.EventInfo = &tstEvent{
		t: t,
		p: dumpFile,
	}
	cfg := &opts.Opts{
		LocalOut:         "/tmp",
		MaxTries:         1,
		RetryDelayConst:  2,
		Compresser:       "xz",
		CompressExt:      "xz",
		CompressionLevel: "-0",
	}
	handleEvent(ei, cfg)
	tlog.Freeze()
	usingLocal := "not using s3 - writing to local file "
	btname := ""
	corename := ""
	for _, l := range strings.Split(tlog.Buf.String(), "\n") {
		//lines begin with a timestamp, so can't use strings.HasPrefix
		i := strings.Index(l, usingLocal)
		if i > 0 {
			fname := strings.TrimPrefix(l[i:], usingLocal)
			t.Log(fname)
			_, err := os.Stat(fname)
			if err == nil {
				if strings.Contains(fname, "backtrace") {
					btname = fname
				} else if strings.Contains(fname, "core") {
					corename = fname
				} else {
					t.Error("unknown file:", fname)
				}
			} else {
				t.Error(err)
			}
		}
	}
	if btname == "" {
		t.Error("missing backtrace file")
	} else {
		err := os.Remove(btname)
		if err != nil {
			t.Error(err)
		}
	}
	if corename == "" {
		t.Error("missing core file")
	} else {
		err := os.Remove(corename)
		if err != nil {
			t.Error(err)
		}
	}
	if t.Failed() {
		t.Logf("log output:\n%s\n", tlog.Buf.String())
	}
}

/*
handleEvent needs a notify.EventInfo. Fortunately
that's an interface so we can easily create our own.
*/
type tstEvent struct {
	t *testing.T
	p string
}

func (te *tstEvent) Event() (e notify.Event) {
	te.t.Error("tstEvent.Event not implemented")
	return
}
func (te *tstEvent) Path() string {
	return te.p
}
func (te *tstEvent) Sys() (i interface{}) {
	te.t.Error("tstEvent.Sys not implemented")
	return
}

var _ notify.EventInfo = &tstEvent{}

func TestNotifySelf(t *testing.T) {
	for _, td := range []struct {
		name string
		want notify.Event
		act  func(t *testing.T, cfg *opts.Opts)
	}{
		{
			name: "rename",
			want: notify.InMoveSelf,
			act: func(t *testing.T, cfg *opts.Opts) {
				err := os.Rename(cfg.WatchDir, cfg.WatchDir+"2")
				if err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "del and link",
			want: notify.InDeleteSelf,
			act: func(t *testing.T, cfg *opts.Opts) {
				err := os.RemoveAll(cfg.WatchDir)
				if err != nil {
					t.Fatal(err)
				}
				err = os.Mkdir(cfg.WatchDir+"2", 0755)
				if err != nil {
					t.Fatal(err)
				}
				err = os.Symlink(cfg.WatchDir+"2", cfg.WatchDir)
				if err != nil {
					t.Fatal(err)
				}
			},
		},
	} {
		t.Run(td.name, func(t *testing.T) {
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

			cfg := &opts.Opts{
				WatchDir: wDir,
			}
			eventChan := make(chan notify.EventInfo, 10)
			defer notify.Stop(eventChan)
			futil.WaitForDir(cfg.WatchedIsMountpoint, cfg.WatchDir)
			if err := notify.Watch(cfg.WatchDir+"/...", eventChan, notify.InCloseWrite, notify.InMovedTo, notify.InMoveSelf, notify.InDeleteSelf); err != nil {
				log.Fatalf("watching %s: %s", cfg.WatchDir, err)
			}
			select {
			case ei := <-eventChan:
				t.Error("unexpected event", ei)
			case <-time.After(time.Second / 10):
			}
			td.act(t, cfg)
			select {
			case <-time.After(time.Second / 10):
				t.Error("expected event")
			case ei := <-eventChan:
				got := evtMask(ei)
				if got&td.want == 0 {
					t.Errorf("got %s, want %s, evt=%#v", got, td.want, ei)
				}
			}
		})
	}
}
