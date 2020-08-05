// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package kver

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/purecloudlabs/gprovision/pkg/log"
)

/*
values from kernel documentation and libmagic src

off val
510 0xAA55
514 HdrS
526	(4 bytes) != 0x0000
526 (2 bytes, little endian) + 0x200 -> start of null-terminated version string
*/

var (
	EBootSig = errors.New("missing 0x55AA boot sig")
	EBadSig  = errors.New("missing kernel header sig")
	EBadOff  = errors.New("null version string offset")
	EBadStr  = errors.New("missing termination in version string")
	EParse   = errors.New("parse error")
)

//Read kernel version string
func GetKDesc(k io.ReadSeeker) (string, error) {
	var buf [1024]byte
	_, err := k.Seek(0, io.SeekStart)
	if err != nil {
		return "", err
	}
	_, err = k.Read(buf[:530])
	if err != nil {
		return "", err
	}
	if !bytes.Equal(buf[510:512], []byte{0x55, 0xaa}) {
		return "", EBootSig
	}
	if string(buf[514:518]) != "HdrS" {
		return "", EBadSig
	}
	if bytes.Equal(buf[526:530], []byte{0, 0, 0, 0}) {
		return "", EBadOff
	}
	off := int64(binary.LittleEndian.Uint16(buf[526:528])) + 0x200
	_, err = k.Seek(off, io.SeekStart)
	if err != nil {
		return "", err
	}
	if _, err := k.Read(buf[:]); err != nil {
		return "", err
	}
	var i int
	var b byte
	for i, b = range buf {
		if b == 0 {
			break
		}
	}
	if i == 1023 {
		return "", EBadStr
	}
	return string(buf[:i]), nil
}

type KInfo struct {
	//2.6.24.111 (bluebat@linux-vm-os64.site) #606 Mon Apr 14 00:06:11 CEST 2014
	//4.19.16-norm_boot (user@host) #300 SMP Fri Jan 25 16:32:19 UTC 2019
	//   release               (builder)                              version
	//maj.min.patch-localver                                      #buildnum SMP buildtime
	Release, Version string //uname -r, uname -v respectfully
	Builder          string //user@hostname in parenthesis, shown by `file` but not `uname`

	//the following are extracted from Release and Version

	BuildNum        uint64    //#nnn in Version, 300 in example above
	BuildTime       time.Time //from Version
	Maj, Min, Patch uint64    //from Release
	LocalVer        string    //from Release
}

const layout = "Mon Jan 2 15:04:05 MST 2006"

//Parse output of GetKDesc
func ParseDesc(desc string) (KInfo, error) {
	var ki KInfo

	//first split at #
	split := strings.Split(desc, "#")
	if len(split) != 2 {
		log.Logf("unable to parse %s, wrong number of '#' chars", desc)
		return KInfo{}, EParse
	}
	ki.Version = "#" + split[1]

	//now split first part into release and builder
	elements := strings.SplitN(split[0], " ", 2)
	if len(elements) > 2 {
		log.Logf("unable to parse %s, wrong number of spaces in release/builder", desc)
		return KInfo{}, EParse
	}
	ki.Release = elements[0]
	if len(elements) == 2 {
		//not sure if this is _always_ present
		ki.Builder = strings.Trim(elements[1], " ()")
	}
	//split build number off version
	elements = strings.SplitN(split[1], " ", 2)
	if len(elements) != 2 {
		log.Logf("unable to parse %s, wrong number of spaces in build/version", desc)
		return KInfo{}, EParse
	}
	i, err := strconv.ParseUint(elements[0], 10, 64)
	if err != nil {
		log.Logf("unable to parse %s, bad uint %s: %s", desc, elements[0], err)
		return KInfo{}, err
	}
	ki.BuildNum = i
	//remove SMP if present
	t := strings.TrimSpace(strings.TrimPrefix(elements[1], "SMP"))
	//parse remainder as time, using reference time
	ki.BuildTime, err = time.Parse(layout, t)
	if err != nil {
		log.Logf("unable to parse %s, bad time %s: %s", desc, t, err)
		return KInfo{}, err
	}
	elements = strings.Split(ki.Release, ".")
	if len(elements) < 3 {
		log.Logf("unable to parse %s, wrong number of dots in release %s", desc, ki.Release)
		return KInfo{}, EParse
	}
	ki.Maj, err = strconv.ParseUint(elements[0], 10, 64)
	if err != nil {
		log.Logf("unable to parse %s, bad uint %s: %s", desc, elements[0], err)
		return KInfo{}, err
	}
	ki.Min, err = strconv.ParseUint(elements[1], 10, 64)
	if err != nil {
		log.Logf("unable to parse %s, bad uint %s: %s", desc, elements[1], err)
		return KInfo{}, err
	}
	elem := strings.SplitN(elements[2], "-", 2)
	ki.Patch, err = strconv.ParseUint(elem[0], 10, 64)
	if err != nil {
		log.Logf("unable to parse %s, bad uint %s: %s", desc, elem[0], err)
		return KInfo{}, err
	}

	elements = strings.SplitN(elements[len(elements)-1], "-", 2)
	if len(elements) > 1 {
		ki.LocalVer = elements[1]
	}
	return ki, nil
}
