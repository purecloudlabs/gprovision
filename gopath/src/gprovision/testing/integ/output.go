// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package integ

import (
	"bufio"
	"bytes"
	"encoding/json"
	"gprovision/pkg/log"
	gtst "gprovision/testing"
	"gprovision/testing/vm"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	fp "path/filepath"
	"strings"
	"time"

	"github.com/u-root/u-root/pkg/qemu"
	"github.com/u-root/u-root/pkg/uroot/logger"
)

func expect(t gtst.TB, vm *qemu.VM, lines ...string) {
	t.Helper()
	for _, l := range lines {
		start := time.Now()
		t.Logf("** waiting for %q **", l)
		err := vm.Expect(l)
		if err == nil {
			t.Logf("^^ found ^^ (+%s)", time.Since(start))
		} else {
			t.Fatal(err)
		}
	}
}

func expectLogContent(t gtst.TB, tmpdir string, uefi bool, strs ...string) {
	rawlog := vm.Rawpath(tmpdir, uefi)
	f, err := os.Open(rawlog)
	if err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if len(strs) == 0 {
			break
		}
		if strings.Contains(line, strs[0]) {
			//remove current line
			strs = strs[1:]
			continue
		}
	}
	if err := scanner.Err(); err != nil {
		t.Error(err)
	}
	if len(strs) > 0 {
		t.Errorf("missing strings in raw log: %#v", strs)
	}
}

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
