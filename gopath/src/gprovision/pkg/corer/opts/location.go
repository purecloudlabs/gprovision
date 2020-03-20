// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package opts

import (
	"bytes"
	"encoding/json"
	"gprovision/pkg/common/strs"
	"gprovision/pkg/log"
	"io/ioutil"
	"net/http"
	"os"
	fp "path/filepath"
	"strings"
	"text/template"
)

type TmplData struct {
	InstanceId, Env, Region  string //AWS
	DevCodeName, SKU, Serial string //hardware
}

//handles location options and populates template data, some of which is also required by s3 upload code
func (opts *Opts) checkLocationOpts(s3location string) {
	if s3location == "help" {
		log.Fatalf(`Template values (note the use of brackets instead of braces):
	-- values for AWS --
		[[.InstanceId]]
		[[.Env]]
		[[.Region]]
	-- values for hardware --
		[[.DevCodeName]]
		[[.SKU]]
		[[.Serial]]`)
	}
	if s3location == "" && opts.LocalOut == "" || s3location != "" && opts.LocalOut != "" {
		log.Fatalf("Requires exactly one of -s3location, -local")
	}
	if opts.LocalOut != "" {
		return
	}

	//template expansion in s3location
	getTmplData(opts)

	//change template chars to [] so as to not interfere with Jinja2
	tmpl, err := template.New("location").Delims("[[", "]]").Parse(s3location)
	if err != nil {
		log.Fatalf("%s", err)
	}
	buf := new(bytes.Buffer)
	err = tmpl.Execute(buf, opts.TmplData)
	if err != nil {
		log.Fatalf("%s", err)
	}
	loc := strings.Trim(strings.TrimPrefix(buf.String(), "s3://"), "/")
	split := strings.SplitN(loc, "/", 2)
	if len(split) == 0 {
		log.Fatalf("Cannot parse s3 bucket/prefix", loc)
	}
	opts.S3bkt = split[0]
	if len(split) == 2 {
		opts.S3prefix = split[1]
	}
	if opts.Verbose {
		log.Logln("computed s3 bucket:", opts.S3bkt)
		log.Logln("computed s3 key prefix:", opts.S3prefix)
	}
}

func getTmplData(opts *Opts) {
	// reading two files into the same struct works because json.Unmarshal only
	// touches elements that match the input... and these two don't overlap
	files := []string{
		fp.Join(strs.ConfDir(), "tags.json"),
		fp.Join(strs.ConfDir(), "platform_facts.json"),
	}
	found := 0
	for _, fname := range files {
		_, err := os.Stat(fname)
		if err == nil {
			found++
		}
	}
	if found == 0 {
		log.Logln("data files not found")
	}
	for _, fname := range files {
		data, err := ioutil.ReadFile(fname)
		if err == nil {
			err = json.Unmarshal(data, &opts.TmplData)
		}
		if err != nil {
			if os.IsNotExist(err) && found > 0 {
				//this file doesn't exist, but the other does - ignore
				continue
			}
			log.Logf("reading %s: %s\n", fname, err)
		}
	}
	if opts.TmplData.Serial == "" && (opts.TmplData.InstanceId == "" || opts.TmplData.Region == "") {
		//probably aws, with no tags.json
		rel, _ := ioutil.ReadFile("/etc/os-release")
		if strings.Contains(string(rel), "Amazon") {
			if opts.TmplData.InstanceId == "" {
				resp, err := http.Get("http://169.254.169.254/latest/meta-data/instance-id")
				//FIXME is err always set for http errors or only for connection errors?
				if err != nil {
					log.Logln("failed to get instance-id from 169.254.169.254")
					return
				}
				body, _ := ioutil.ReadAll(resp.Body)
				opts.TmplData.InstanceId = string(body)
			}
			if opts.TmplData.Region == "" {
				resp, err := http.Get("http://169.254.169.254/latest/meta-data/placement/availability-zone")
				//FIXME is err always set for http errors or only for connection errors?
				if err != nil {
					log.Logln("failed to get az from 169.254.169.254")
					return
				}
				body, _ := ioutil.ReadAll(resp.Body)
				az := string(body)
				if len(az) < 2 {
					log.Logln("invalid az:", az)
					return
				}
				region := az[:len(az)-1]
				opts.TmplData.Region = region
			}
		}
	}
}
