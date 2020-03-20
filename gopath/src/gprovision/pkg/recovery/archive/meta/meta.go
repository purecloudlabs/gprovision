// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package meta

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	"gprovision/pkg/common/strs"
	"gprovision/pkg/log"
	"io"
	fp "path/filepath"
	"strings"
	"time"
)

const (
	oneM = 1024 * 1024 //arbitrary data read limit of 1 MB
)

var (
	//remove leading slash because that will not be present in tar
	MetaPath = strings.TrimPrefix(fp.Join(strs.ConfDir(), "imgmeta.json"), "/")
)

// ImgMeta is embedded in a .upd file, and describes the image and CI job.
// CI env vars are the source of most of these values.
type ImgMeta struct {
	BinVer    string
	BinTime   string
	ImportJob string
	ImgJob    string
	ImgName   string
	Stream    string
}

//Disktag returns disktag name for given platform.
func (im *ImgMeta) Disktag(plat string) string {
	if strings.Count(im.ImgName, "HW") != 1 {
		log.Logf("meta: invalid ImgName %s", im.ImgName)
		return ""
	}
	tmp := strings.Replace(im.ImgName, "HW", plat, 1)
	return strings.TrimSuffix(tmp, "upd") + "disktag"
}

func (im *ImgMeta) String() string {
	//pretty-print build time, if possible
	var ts string
	t, err := time.Parse("0601021504", im.BinTime)
	if err == nil {
		ts = t.Format(time.RFC3339)
	} else {
		ts = "(PARSE ERR) " + im.BinTime
	}
	str := fmt.Sprintln("BinVer:   ", im.BinVer)
	str += fmt.Sprintln("BinTime:  ", ts)
	str += fmt.Sprintln("ImportJob:", im.ImportJob)
	str += fmt.Sprintln("ImgJob:   ", im.ImgJob)
	str += fmt.Sprintln("ImgName:  ", im.ImgName)
	if im.Stream != "main_systest" {
		str += fmt.Sprintln("Alt Tier: ", im.Stream)
	}
	return str
}

// ReadRaw reads a metadata file embedded at the beginning of a .upd (tar.xz)
// archive, returning the raw data, which is encoded as json. See also: Read()
//
// This metadata contains more information than available in the (current)
// filename format. Ensuring a file comes first in a tarball is as simple as
// listing it first on the tar command line; it has the side effect of causing
// the file to appear twice in the archive but this seems to cause no problems.
// I think if the two were different that the 2nd would overwrite the first,
// given the historical use of tar (Tape ARchive).
func ReadRaw(upd string) ([]byte, error) {
	xz, cleanup, err := unxz(upd)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	tr := tar.NewReader(io.LimitReader(xz, oneM))
	var h *tar.Header
	for {
		h, err = tr.Next()
		if err != nil {
			return nil, err
		}
		if h.FileInfo().IsDir() {
			continue
		}
		if strings.HasSuffix(h.Name, MetaPath) {
			log.Logf("meta: found %s", h.Name)
			break
		}
		log.Logf("meta: out-of-order file %s", h.Name)
	}
	buf := make([]byte, oneM)
	n, err := tr.Read(buf)
	if err != nil && err != io.EOF {
		return nil, err
	}
	buf = buf[:n]
	return buf, nil
}

// Read is like ReadRaw but returns an ImgMeta struct.
func Read(upd string) (*ImgMeta, error) {
	buf, err := ReadRaw(upd)
	if err != nil {
		return nil, err
	}
	meta := &ImgMeta{}
	err = json.Unmarshal(buf, meta)
	if err != nil {
		return nil, err
	}
	return meta, nil
}
