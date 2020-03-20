// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package fileutil

import (
	"gprovision/pkg/hw/cfa"
	"gprovision/pkg/log"
	"os"
	"time"
)

// Called from a goroutine, updates lcd with size of a file.
// Use with download or decompress operation; close(done) to stop.
func ShowProgress(done chan struct{}, activityDesc, path string) {
	if cfa.DefaultLcd == nil {
		//wait for channel to close, so our behavior matches that with lcd present
		<-done
		return
	}
	noErr := true //only log stat error once
	sp := cfa.Spinner{
		Msg: activityDesc + "...",
		Lcd: cfa.DefaultLcd,
	}
	if err := sp.Display(); err != nil {
		log.Logf("progress: %s", err)
	}
	for {
		time.Sleep(1 * time.Second)
		select {
		case <-done:
			return
		default:
		}
		fi, err := os.Stat(path)
		if err != nil {
			if noErr {
				log.Logf("ShowProgress: Stat() reports %s", err)
				noErr = false
			}
			log.Msgf("%s...", activityDesc)
			continue
		}
		size := fi.Size()
		if size == 0 {
			sp.Next()
		} else {
			log.Msgf("%s... %dM", activityDesc, size/(1024*1024))
		}
	}
}
