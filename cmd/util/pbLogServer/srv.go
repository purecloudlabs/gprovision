// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package main

import (
	"flag"

	"github.com/purecloudlabs/gprovision/pkg/log"
	"github.com/purecloudlabs/gprovision/pkg/oss/pblog/server"
)

var dbfile string

func main() {
	log.AddConsoleLog(0)
	log.FlushMemLog()
	flag.StringVar(&dbfile, "db", "./pb.db", "path to database")
	server.Flags()
	srvr := server.NewServer(dbfile)
	log.Logf("starting server on %s...", server.Port)

	srvr.Serve()
}
