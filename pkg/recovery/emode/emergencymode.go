// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

//Package emode checks any emergency-mode files to determine whether they are updates, json data, or unknown.
package emode

import (
	"fmt"
	"os"
	fp "path/filepath"
	"time"

	"github.com/purecloudlabs/gprovision/pkg/fileutil"
	"github.com/purecloudlabs/gprovision/pkg/hw/cfa"
	hk "github.com/purecloudlabs/gprovision/pkg/init/housekeeping"
	"github.com/purecloudlabs/gprovision/pkg/log"
)

const (
	jsonMaxSize = 1024 * 1024 //arbitrary 1M limit
)

// CheckForEmergency() looks for emergency-mode file(s) and detects type. Only one of two return variables
// is used at a time. If efiles contains multiple files, chooses first that's valid
func CheckForEmergency(efiles []string) (jsons []string, image string) {
	if len(efiles) == 0 {
		return
	}
	log.Msgf("Emergency-mode file(s) found. Checking...")
	time.Sleep(time.Second) //so that the user has time to see the message

	//volume containing emergency image will be unmounted after we're done reading the image

	var imgs []string
	imgs, jsons = checkFiles(efiles)
	if len(imgs) > 0 {
		image = imgs[0]
		if len(imgs) > 1 {
			msg := fmt.Sprintf("At least 2 emergency images found. In 20s: blindly choosing 1st, %s.", fp.Base(image))
			log.Log(msg)
			_ = cfa.DefaultLcd.BlinkMsg(msg, cfa.Fade, time.Second*2, time.Second*20)
		}
		jsons = nil
		return
	}
	if len(jsons) == 0 {
		MustPowerCycle(ReasonCorruption)
	}
	return
}

func checkFiles(files []string) (imgs, jsons []string) {
	for _, fname := range files {
		fi, err := os.Stat(fname)
		if err != nil {
			log.Logf("Failed to stat %s: %s", fname, err)
			continue
		}
		size := fi.Size()
		if fileutil.IsXZSha256(fname) {
			imgs = append(imgs, fname)
			continue
		}
		if size <= jsonMaxSize {
			jsons = append(jsons, fname)
		} else {
			//too big to be json, but lacks appropriate signature for image
			log.Logf("Ignoring corrupt (?) emergency file %s", fname)
		}
	}
	return
}

const (
	ReasonCorruption = "errors were encountered (file corruption?)"
	ReasonNoMatch    = "none are for this device"
)

//never returns
func MustPowerCycle(reason string) {
	hk.Preboots.Perform(false)
	time.Sleep(time.Second)
	a := "Emergency mode file(s) located, but "
	b := ". Remove external media and power cycle unit."
	msg := a + reason + b
	log.Log(msg)
	_, _ = cfa.FindWithRetry() //uninitialized by Preboot functions
	for {
		_ = cfa.DefaultLcd.BlinkMsg(msg, cfa.Fade, 4*time.Second, 48*time.Hour)
	}
}
