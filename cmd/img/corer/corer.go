// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

// Service corer listens for fs events on given dir, and if the events are
// for a coredump, it creates a backtrace via gdb. The backtrace as well as the
// core are uploaded to s3. See packages under github.com/purecloudlabs/gprovision/pkg/corer for details.
package main

import (
	"strings"
	"time"

	"github.com/purecloudlabs/gprovision/pkg/corer/backtrace"
	"github.com/purecloudlabs/gprovision/pkg/corer/opts"
	"github.com/purecloudlabs/gprovision/pkg/corer/stream"
	futil "github.com/purecloudlabs/gprovision/pkg/fileutil"
	"github.com/purecloudlabs/gprovision/pkg/log"
	"github.com/purecloudlabs/gprovision/pkg/log/flags"

	"github.com/rjeczalik/notify"
	"golang.org/x/sys/unix"
)

//in any binary with main.buildId string, it is set at compile time to $BUILD_INFO
var buildId string

func main() {
	log.AddConsoleLog(flags.NA)
	log.FlushMemLog()
	log.Logf("buildId: %s", buildId)

	opts := opts.HandleArgs()
	if opts.Analyze != "" {
		backtrace.Create(opts, opts.Analyze, stream.Write)
		return
	}

	//set up fs event notifications
	eventChan := make(chan notify.EventInfo, 1)
	watch(opts, eventChan)
	defer notify.Stop(eventChan)

	//handle fs events
	for {
		ei := <-eventChan
		if opts.Verbose {
			log.Logln("Got event:", ei)
		}
		mask := evtMask(ei)
		if (mask&notify.InMoveSelf != 0) || (mask&notify.InDeleteSelf != 0) {
			//dir has moved
			log.Log("dir has been (re)moved, recreating watch...")
			notify.Stop(eventChan)
			watch(opts, eventChan)
			continue
		}
		go handleEvent(ei, opts)
	}
}

func evtMask(ei notify.EventInfo) notify.Event {
	eType := ei.Sys().(*unix.InotifyEvent)
	return notify.Event(eType.Mask)
}

func watch(cfg *opts.Opts, eventChan chan notify.EventInfo) {
	futil.WaitForDir(cfg.WatchedIsMountpoint, cfg.WatchDir)
	if err := notify.Watch(cfg.WatchDir+"/...", eventChan, notify.InCloseWrite, notify.InMovedTo, notify.InMoveSelf, notify.InDeleteSelf); err != nil {
		log.Fatalf("watching %s: %s", cfg.WatchDir, err)
	}
}

func handleEvent(ei notify.EventInfo, cfg *opts.Opts) {
	//is it a core?
	path := ei.Path()
	if !strings.HasSuffix(path, ".core") {
		if cfg.Verbose {
			log.Logln("ignoring event for", path)
		}
		return
	}
	log.Logln("handling event for", path)
	backtrace.Upload(cfg, path)
	for try := 0; try < cfg.MaxTries; try++ {
		err := stream.Compress(cfg, path, stream.Write)
		if err != nil {
			log.Logf("upload %d of %s failed: %s\n", try, path, err)
			//TODO: if too many failures, upload without compression??
			if try < cfg.MaxTries-1 {
				time.Sleep(time.Duration(cfg.RetryDelayConst*try) * time.Second)
			}
		} else {
			break
		}
	}
	log.Logln("done handling event for", path)
}
