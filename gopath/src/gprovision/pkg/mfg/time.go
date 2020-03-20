// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package mfg

import (
	"gprovision/pkg/common/rkeep"
	"gprovision/pkg/log"
	"os/exec"
	"time"
)

const (
	queryMax   = 5 * time.Second
	maxQueries = 10
)

// Query server for time with retry. Retry until round trip
// is < queryMax, or # tries > maxQueries.
func serverTimeUTC( /*server string*/ ) (local time.Time, serverTime string) {
	tries := 0
	for tries < maxQueries {
		start := time.Now()
		serverTime = rkeep.GetTime()
		end := time.Now()
		delta := end.Sub(start)
		if delta < queryMax {
			local = end
			return
		}
		tries++
		log.Logf("Max %s for time query; this query took %s. Sleeping %ds, then retrying...", queryMax, delta, 5*tries)
		time.Sleep(5 * time.Second * time.Duration(tries))
	}
	log.Fatalf("Too many tries acquiring accurate time")
	return
}

func setTime(t string) {
	set := exec.Command("date", "-u", "-s", t)
	_, success := log.Cmd(set)
	if !success {
		log.Fatalf("Failed to set system time")
	}
}

func SetTimeFromServer() {
	localT, serverT := serverTimeUTC()
	setTime(serverT)
	/* server time is handled in string form so it can be passed directly to
	   the date command. Now, convert to time.Time so we can log the delta.
	*/
	parsed, err := time.Parse("2006-01-02 15:04:05", serverT)
	if err != nil {
		log.Fatalf("error parsing server time: %s", err)
	}
	log.Logf("Time adjustment:\nlocal system time was   %s\nnow matches server time %s", localT.UTC().Format(time.UnixDate), parsed.Format(time.UnixDate))
	log.Msgf("Time adjusted by %s", localT.Sub(parsed))
}
