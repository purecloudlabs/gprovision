// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"

	"github.com/purecloudlabs/gprovision/pkg/log"
	"github.com/purecloudlabs/gprovision/pkg/log/flags"
	"github.com/purecloudlabs/gprovision/pkg/recovery/archive/meta"
)

func init() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", os.Args[0])
		fmt.Fprintf(flag.CommandLine.Output(), "%s [-raw] [-v] PRODUCT.Os.Plat.blah.upd\n", os.Args[0])
		fmt.Fprintf(flag.CommandLine.Output(), "  Reads metadata from .upd file.\n")
		flag.PrintDefaults()
	}
}

var buildId string

func main() {
	raw := flag.Bool("raw", false, "output raw data (json)")
	v := flag.Bool("v", false, "verbose")
	flag.Parse()

	logflags := flags.EndUser | flags.Fatal
	if *v {
		logflags = flags.NA
		log.Logf("buildId: %s", buildId)
	}
	log.AddConsoleLog(logflags)
	log.FlushMemLog()

	if flag.NArg() != 1 {
		log.Fatalf("need exactly one arg")
	}
	var err error //for common error handling below
	if *raw {
		var json []byte
		json, err = meta.ReadRaw(flag.Arg(0))
		if err == nil {
			fmt.Println(string(json))
		}
	} else {
		var mdata *meta.ImgMeta
		mdata, err = meta.Read(flag.Arg(0))
		if err == nil {
			fmt.Print(mdata)
		}
	}
	if err == io.ErrUnexpectedEOF {
		log.Fatalf("metadata not found at beginning of file")
	}
	if err != nil {
		log.Fatalf("error %s", err)
	}

	if runtime.GOOS == "windows" {
		fmt.Println("press any key to continue...")
		entered := ""
		fmt.Scanln(&entered)
	}
}
