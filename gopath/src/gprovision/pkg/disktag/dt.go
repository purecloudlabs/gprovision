// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

//Package disktag encodes and decodes disktags.
package disktag

import (
	"gprovision/pkg/log"
	"io/ioutil"
	fp "path/filepath"
	"runtime"
	"strings"
)

var tag string
var platformName string

func SetPlatform(plat string) {
	platformName = plat
}

func Set(t string) {
	tag = t + ".disktag"

}

func Get() string {
	return strings.TrimSuffix(tag, ".disktag")
}

//convert image prefix (w/o .upd) to disktag prefix
func ImgToDTag(img string) (dtag string) {
	dtag = strings.Replace(img, "HW", platformName, 1)
	return
}

// write disktag
func Write(root string) {
	if len(tag) == 0 {
		panic("cannot write 0-length disktag")
	}
	log.Logf("writing %s", tag)
	err := ioutil.WriteFile(fp.Join(root, tag), nil, 0444)
	if err != nil {
		log.Logf("Error writing disk tag: %s", err)
	}
}

// read disktag from disk
func Read(root string) string {
	tags, err := fp.Glob(fp.Join(root, "*.disktag"))
	if err != nil {
		log.Logf("Error %s searching for disktag", err)
		return ""
	}
	if len(tags) != 1 {
		//we have a defective image but don't know what one. eep!
		log.Logf("Error, expected one disktag. found %d: %v", len(tags), tags)
		return ""
	}
	return fp.Base(tags[0])
}

// read disktag, return 3rd component (platform) or empty string
func Platform(root string) (plat string) {
	tag := Read(root)
	if tag == "" && root == "" {
		if runtime.GOOS == "windows" {
			root = "C:"
		} else {
			root = "/"
		}
		tag = Read(root)
	}
	components := strings.Split(tag, ".")
	if len(components) > 2 {
		plat = components[2]
	} else {
		log.Logf("cannot parse %s", tag)
	}
	return
}
