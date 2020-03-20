// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

//Package backtrace creates backtraces from coredumps.
package backtrace

import (
	"archive/zip"
	"bytes"
	"fmt"
	"gprovision/pkg/corer/opts"
	"gprovision/pkg/corer/stream"
	"gprovision/pkg/log"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	fp "path/filepath"
	"runtime"
	"strings"
)

func Upload(cfg *opts.Opts, core string) {
	Create(cfg, core, stream.LocalCopy)
}

//use gdb to create backtrace, pass to nextFn
func Create(cfg *opts.Opts, core string, nextFn stream.NextFn) {
	if cfg.Nobt {
		return
	}

	//determine executable which created backtrace
	exe := FindExe(core)
	if exe == "" {
		log.Logln("failed to determine exe, cannot create backtrace for", exe)
		return
	}
	//gdb outputs will go in zip archive written to buf
	buf, err := zippedBacktrace(exe, core, cfg.Verbose)
	if err != nil {
		log.Logln(err)
		return
	}
	zipname := strings.TrimSuffix(fp.Base(core), ".core") + "_backtrace.zip"

	//try to write the zip to a local file in same dir as core, then upload
	// in the unlikely event file creation fails, upload from the buffer
	localpath := fp.Join(fp.Dir(core), zipname)
	err = nextFn(cfg, localpath, buf)
	if err != nil {
		log.Logln("processing backtrace failed:", err)
	}
}

const extraDir = "extras/"

func zippedBacktrace(exe, core string, verbose bool) (buf *bytes.Buffer, err error) {
	buf = new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	if verbose {
		log.Logln("collecting core information via gdb")
	}
	args := []string{exe, core}
	cmds := []string{
		"bt",
		"thread apply all bt",
		"info sharedlib",
	}
	//zip files in zw, using cmds above as file names (substituting underscore for space)
	zipOuts := func(files []string, extras []memFile) (err error) {
		for i, tmpfile := range files {
			name := strings.Replace(cmds[i], " ", "_", -1) + ".txt"
			var in *os.File
			var out io.Writer
			in, err = os.Open(tmpfile)
			if err != nil {
				log.Logln("adding file to zip:", err)
				continue
			}
			defer in.Close()
			out, err = zw.Create(name)
			if err != nil {
				log.Logln("adding file to zip:", err)
				continue
			}
			_, err = io.Copy(out, in)
			if err != nil {
				log.Logln("adding file to zip:", err)
				continue
			}
		}
		if len(extras) > 0 {
			txt := extraDir + " subdir contains information about gdb's resource usage\n" +
				"while processing the core. Peak usage is found in the rusage file.\n" +
				"The meminfo files are collected as gdb starts, on one second\n" +
				"intervals thereafter, and when gdb exits. Note that the total\n" +
				"number of meminfo files is limited, currently to 20."
			legend, err := zw.Create(extraDir + "readme.txt")
			if err == nil {
				if _, err := legend.Write([]byte(txt)); err != nil {
					log.Logf("writing legend: %s", err)
				}
			} else {
				log.Logln("adding file to zip:", err)
			}
		}
		for _, extra := range extras {
			var out io.Writer
			out, err = zw.Create(extraDir + extra.name)
			if err != nil {
				log.Logln("adding file to zip:", err)
				continue
			}
			_, err = out.Write(extra.content)
			if err != nil {
				log.Logln("adding file to zip:", err)
				continue
			}
		}
		return
	}
	err = gdbMultiCmd(args, cmds, zipOuts, false, verbose)
	if err != nil {
		return
	}
	zw.Close()
	return
}

type memFile struct {
	name    string
	content []byte
}
type gdbOutputProcessor func(files []string, extras []memFile) error

//run multiple gdb commands, send each's output to a file. process those files with processOuts func. Files will be deleted after processor returns unless nodelete is true.
func gdbMultiCmd(gdbArgs, gdbCmds []string, processOuts gdbOutputProcessor, nodelete, stats bool) (err error) {
	cmdFile, err := ioutil.TempFile("", "gdbCmds")
	if err != nil {
		return fmt.Errorf("creating gdb command file: %s", err)
	}
	defer os.Remove(cmdFile.Name())
	defer cmdFile.Close()

	var files []string
	for _, cmd := range gdbCmds {
		f, err := ioutil.TempFile("", "gdbCoreOut")
		if err != nil {
			return fmt.Errorf("creating gdb output file: %s", err)
		}
		files = append(files, f.Name())
		if !nodelete {
			defer os.Remove(f.Name())
		}
		f.Close()
		fmt.Fprintf(cmdFile, "set logging on %s\n", f.Name())
		fmt.Fprintf(cmdFile, "%s\n", cmd)
		fmt.Fprintf(cmdFile, "set logging off\n")
	}
	cmdFile.Close()

	done := make(chan struct{})
	collectedInfo := make(chan []byte, 20) //enough for ~19s; in testing, only takes a few s
	if stats {
		go collectMemInfo(done, collectedInfo)
		runtime.Gosched() //should be enough to get statistics captured before gdb starts
	}

	gdb := exec.Command("gdb", "-q", "-x", cmdFile.Name())
	gdb.Args = append(gdb.Args, gdbArgs...)

	err = gdb.Run()
	if err != nil {
		return fmt.Errorf("collecting info from gdb: %s", err)
	}
	close(done)
	var extraData []memFile
	if stats {
		extraData = append(extraData, memFile{name: "rusage.txt",
			content: procRUsage(gdb),
		})
		i := 0
		for stat := range collectedInfo {
			extraData = append(extraData, memFile{
				name:    fmt.Sprintf("proc-meminfo_%d.txt", i),
				content: []byte(stat),
			})
			i++
		}
	}
	return processOuts(files, extraData)
}

func zipOutput(zw *zip.Writer, name string, cmd *exec.Cmd) {
	f, err := zw.Create(name)
	if err != nil {
		log.Logln("failed to create file in zip:", err)
		return
	}
	cmd.Stdout = f
	err = cmd.Run()
	if err != nil {
		log.Logf("running %v: %s\n", cmd.Args, err)
	}
}

//TODO limit  gdb cpu/memory usage
