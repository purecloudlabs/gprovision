// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package server

// cause bindata.go to be generated from files in the data dir
//go:generate ../../../../bin/go-bindata -prefix=data/ -pkg=$GOPACKAGE data/

import (
	"fmt"
	"html/template"
	"net/http"
	"time"

	"github.com/purecloudlabs/gprovision/pkg/oss/pblog/pb"
)

type devStruct struct {
	SN       string
	Macs     pb.MACs
	IpmiMacs pb.MACs
	Entries  pb.LogEvents
}

var viewTmpl *template.Template
var bgData, cssData []byte

func init() {
	vt, _ := Asset("view.tmpl.html")
	funcMap := template.FuncMap{
		"tsStr": tsStr,
	}
	viewTmpl = template.Must(template.New("view").Funcs(funcMap).Parse(string(vt)))
	bgData, _ = Asset("bg.svg")
	cssData, _ = Asset("main_css")
}

func css(w http.ResponseWriter, req *http.Request) {
	now := time.Now()
	w.Header().Set("Content-Type", "text/css")
	_, err := w.Write(cssData)
	if err != nil {
		fmt.Println("error", err)
	}
	if now.Month() == 4 && (now.Day() == 1 || (now.Day() < 4 && now.Weekday() == time.Monday)) {
		//correct for foolish axial tilt in 4th month
		_, err = w.Write([]byte(`body{transform:rotate(.2deg);margin-left:15em;}`))
		if err != nil {
			fmt.Println("error", err)
		}
	}
}

func bg(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "image/svg+xml")
	_, err := w.Write(bgData)
	if err != nil {
		fmt.Println("error", err)
	}
}

func (a *allInOneSrvr) view(w http.ResponseWriter, req *http.Request) {
	var ds devStruct
	var err error
	ds.SN = req.URL.Query().Get(":sn")
	ds.Macs, err = a.store.RetrieveMacs(ds.SN)
	if err != nil {
		fmt.Fprintf(w, "<br>ERROR %s", err)
	}
	ds.IpmiMacs, err = a.store.RetrieveIpmiMacs(ds.SN)
	if err != nil {
		fmt.Fprintf(w, "<br>ERROR %s", err)
	}
	ds.Entries, err = a.store.RetrieveLog(ds.SN)
	if err != nil {
		fmt.Fprintf(w, "<br>ERROR %s", err)
	}
	err = viewTmpl.Execute(w, ds)
	if err != nil {
		fmt.Fprintf(w, "<br>error: %s", err)
	}
}
