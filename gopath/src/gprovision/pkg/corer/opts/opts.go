// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

//Package opts parses flags and stores config used elsewhere in corer.
package opts

import (
	"flag"
	"gprovision/pkg/common/strs"
	"gprovision/pkg/log"
	"os"
	"os/exec"
)

type Opts struct {
	WatchDir, S3bkt, S3prefix, LocalOut       string
	CompressExt, Compresser, CompressionLevel string
	Nobt, Verbose, WatchedIsMountpoint        bool
	MaxTries                                  int
	RetryDelayConst                           int
	Analyze                                   string
	TmplData                                  TmplData //used for s3 uri, and region is reused by s3 upload code
}

func HandleArgs() (opts *Opts) {
	opts = new(Opts)
	s3location := ""
	flag.BoolVar(&opts.Verbose, "v", false, "verbose")
	flag.StringVar(&opts.WatchDir, "watchDir", os.Getenv(strs.CoreEnv()),
		"Dir to watch, default "+strs.CoreEnv()+". On startup waits for this dir to be created. See also: -mount")
	flag.BoolVar(&opts.WatchedIsMountpoint, "mount", false,
		"If true, on startup wait until watchDir is a mountpoint.")
	flag.StringVar(&s3location, "s3location", "",
		"Upload output file(s) to this S3 location.\nTemplatable; use -s3location=help to list available fields.\nExample: s3://somebucket/blah/blah/[[.InstanceId]]/cores/")
	flag.StringVar(&opts.LocalOut, "local", "",
		"Instead of uploading, write files to this dir. Conflicts with -s3location.")
	flag.BoolVar(&opts.Nobt, "nobt", false, "Skip backtrace")
	flag.StringVar(&opts.CompressExt, "compression", "xz",
		"Compression and file extension to use.\nEmpty string for no compression.\nAllowed values: 'xz','gz',''")
	flag.StringVar(&opts.CompressionLevel, "cmpLevel", "-0",
		"Compression level arg passed to compresser. Recommend 0 for xz.")
	flag.StringVar(&opts.Analyze, "analyze", "",
		"Path to core; run gdb on this core, writing output to zip, and exit.\nRequires -s3location or -local. Conflicts with -nobt.\nIgnores -watchDir and compression opts.")
	flag.Parse()
	if opts.WatchDir == "" && opts.Analyze == "" && s3location != "help" {
		log.Fatalf("%s env var is absent or -watchDir='' was specified", strs.CoreEnv())
	}
	opts.checkLocationOpts(s3location)
	opts.checkGdbOpts()
	opts.checkCompressOpts()
	opts.MaxTries = 10
	opts.RetryDelayConst = 5
	return
}

func (cfg *Opts) checkCompressOpts() {
	if cfg.CompressExt != "" {
		if cfg.CompressExt != "xz" && cfg.CompressExt != "gz" {
			log.Fatalf("unsupported compression")
		}
		cmd := "xz"
		if cfg.CompressExt == "gz" {
			cmd = "gzip"
			if cfg.CompressionLevel == "-0" {
				//level valid for xz only
				cfg.CompressionLevel = "-2"
			}
		}

		path, err := exec.LookPath(cmd)
		if err != nil {
			log.Fatalf(cmd, "not found in path:", err)
		}
		cfg.Compresser = path
	}
}
func (cfg *Opts) checkGdbOpts() {
	if !cfg.Nobt {
		err := exec.Command("gdb", "--version").Run()
		if err != nil {
			log.Logln("Cannot exec gdb - backtraces unavailable:", err)
			cfg.Nobt = true
		}
	}
	if cfg.Analyze != "" {
		if cfg.Nobt {
			log.Fatalf("gdb is required for analysis.")
		}
	}
}
