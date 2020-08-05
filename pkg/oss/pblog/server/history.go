// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package server

import (
	"fmt"
	"net/http"
	"os"
	fp "path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/purecloudlabs/gprovision/pkg/oss/pblog/pb"
)

const timeFormat = "01/02/2006-03:04:05 PM"

type histEntry struct {
	sn       string
	t        time.Time
	state    pb.ProcessState
	platform string
}
type hEntries []*histEntry
type activityHistory struct {
	sync.Mutex
	entries hEntries
}

func (e *histEntry) setStage(state pb.ProcessState, now time.Time) {
	e.t = now
	e.state = state
}
func (e *histEntry) setPlat(plat string, now time.Time) {
	e.t = now
	e.platform = plat
}

const maxHistory = 100

//update or insert an item in activityHistory
//new or modified items are at beginning of list
//limits size of list to maxHistory
func (ah *activityHistory) getent(sn string) *histEntry {
	var entry *histEntry
	var i int
	found := false
	for i, entry = range ah.entries {
		if entry.sn == sn {
			ah.entries = append(ah.entries[:i], ah.entries[i+1:]...)
			ah.entries = append(hEntries{entry}, ah.entries...)
			found = true
			break
		}
	}
	if !found {
		entry = &histEntry{sn: sn}
		ah.entries = append(hEntries{entry}, ah.entries...)
	}
	if len(ah.entries) > maxHistory {
		ah.entries = ah.entries[:maxHistory]
	}
	return entry
}

func (e histEntry) toHtml(now time.Time) (s string) {
	s = "<tr>" + tsToHtml(e.t, now, e.sn)
	format := "<td class='state'>%s</td><td class='plat'>%s</td></tr>"
	s += fmt.Sprintf(format, e.state, e.platform)
	return

}
func tsToHtml(t, now time.Time, sn string) (s string) {
	entryFormat := "<td><time class='%s'>%s</time></td><td class='serial %s'><a href='/view/%s/'>%s</a></td>"
	var timeClass, serialClass string //used to hilight certain time ranges or serials
	if t.Add(time.Hour * 24).After(now) {
		timeClass = "today"
	}
	if strings.HasPrefix(string(sn), "UNKNOWN_SN") {
		serialClass = "unknown"
	}
	s = fmt.Sprintf(entryFormat, timeClass, t.Format(timeFormat), serialClass, sn, sn)
	return
}

var BuildId string

//prints end of html for /recent, including build id
func tail() string {
	return fmt.Sprintf("</table><hr/><div class=buildId>%s: %s</div><br></body></html>", fp.Base(os.Args[0]), BuildId)
}

func (h *activityHistory) toHtml() (s string) {
	now := time.Now()
	s = `<html><head><title>&#127355; Recent Activity</title><link href="/css" rel="stylesheet" type="text/css"></head>
	<body>Displays activity for up to 100 devices<br><a href=/recent/>Refresh</a><br>
	<table class=history><tr><th><time>Time</time></th><th class=serial>Serial</th><th class='state'>Last Known State</th><th class='plat'>Platform</th></tr>`
	h.Lock()
	defer h.Unlock()
	for _, e := range h.entries {
		s += e.toHtml(now)
	}
	s += tail()
	return
}

// handle /recent/
func (a *allInOneSrvr) recentHist(w http.ResponseWriter, req *http.Request) {
	_, err := w.Write([]byte(a.ah.toHtml()))
	if err != nil {
		fmt.Println("error writing page:", err, req)
	}
}

func (a *allInOneSrvr) unitState(w http.ResponseWriter, req *http.Request) {
	sn := req.URL.Query().Get(":sn")
	a.ah.Lock()
	defer a.ah.Unlock()
	for _, e := range a.ah.entries {
		if e.sn == sn {
			_, err := w.Write([]byte(e.state.String()))
			if err != nil {
				fmt.Println("error writing unit state page:", err, req)
			}
			return
		}
	}
	w.WriteHeader(404)
	fmt.Fprintf(w, "sn not found: %q\n%#v", sn, a.ah.entries)
}
