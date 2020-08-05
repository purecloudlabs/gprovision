// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package integ

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	fp "path/filepath"
	"strings"

	"github.com/u-root/u-root/pkg/qemu"
	"github.com/u-root/u-root/pkg/uroot/logger"

	"github.com/purecloudlabs/gprovision/pkg/log"
	gtst "github.com/purecloudlabs/gprovision/testing"
)

// write log file from vm, can only be called in a subtest due to log
// naming logic
func subtestLogfile(t gtst.TB, qopts *qemu.Options, tmpdir string) string {
	lfile := fp.Join(tmpdir, fp.Base(t.Name()))
	t.Logf("vm output will go to %s", lfile)
	logfile(t, qopts, lfile)
	return lfile
}
func logfile(t gtst.TB, qopts *qemu.Options, lfile string) {
	out, err := os.Create(lfile)
	if err != nil {
		t.Fatal(err)
	}
	qopts.SerialOutput = out
}

//use in conjunction with logfile / LClogfile
func readOutLfile(t gtst.TB, lfile string) {
	content, err := ioutil.ReadFile(lfile)
	if err != nil {
		t.Errorf("can't read log file %s: %s", lfile, err)
	} else {
		content = bytes.Replace(content, []byte{033}, []byte{'~'}, -1)
		lines := strings.Split(string(content), "\n")
		l := len(lines)
		if l > 50 {
			lines = lines[l-50:]
		}
		t.Logf("tail of vm output:\n******\n%s\n******\n", strings.Join(lines, "\n"))
	}
}

// for ReadJson/WriteJson
type jrw struct {
	Cmd  []string
	Port int
}

func WriteJson(dir string, cmd *exec.Cmd, port int) {
	j := jrw{
		Port: port,
		Cmd:  cmd.Args,
	}
	data, err := json.Marshal(j)
	if err != nil {
		log.Fatalf("marshalling: %s", err)
	}
	err = ioutil.WriteFile(fp.Join(dir, "replay.json"), data, 0644)
	if err != nil {
		log.Fatalf("writing: %s", err)
	}
}

func ReadJson(dir string) (cmd []string, port int) {
	data, err := ioutil.ReadFile(fp.Join(dir, "replay.json"))
	if err != nil {
		log.Fatalf("reading: %s", err)
	}
	j := &jrw{}
	err = json.Unmarshal(data, j)
	if err != nil {
		log.Fatalf("unmarshalling: %s", err)
	}
	return j.Cmd, j.Port
}

//NoEscape is an io.WriteCloser that filters out escape chars, replacing with '~'
type NoEscape struct {
	Out io.WriteCloser
}

var _ io.WriteCloser = (*NoEscape)(nil)

func (ne *NoEscape) Close() error { return ne.Out.Close() }
func (ne *NoEscape) Write(b []byte) (int, error) {
	return ne.Out.Write(bytes.Replace(b, []byte{033}, []byte{'~'}, -1))
}

//some u-root functions take a Logger interface. This statisfies that interface,
//without use/redirection of std log.
type UrootLoggerAdapter struct{}

var _ logger.Logger = (*UrootLoggerAdapter)(nil)

func (*UrootLoggerAdapter) Printf(format string, v ...interface{}) {
	log.Logf(format, v...)
}
func (*UrootLoggerAdapter) Print(v ...interface{}) {
	log.Logln(v...)
}
