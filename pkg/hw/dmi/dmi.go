// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

//Package dmi reads DMI (aka SMBIOS) data via dmidecode.
package dmi

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/purecloudlabs/gprovision/pkg/log"
)

type DmiStrMap map[string]string
type DmiTypeMap map[int][]byte

//cache data so it's not necessary to call dmidecode over and over. has the side effect of making mocking easy.
type dmiCache struct {
	strings   DmiStrMap
	types     DmiTypeMap
	onlyCache bool //use with TestingMock() - only allow cache lookups, no calls to dmidecode
}

var cache dmiCache

func init() {
	cache = dmiCache{
		strings: DmiStrMap{},
		types:   DmiTypeMap{},
	}
}

//effectively mocks this package by pre-populating the cache and only allowing cache lookups
func TestingMock(s DmiStrMap, t DmiTypeMap) {
	cache = dmiCache{
		strings:   s,
		types:     t,
		onlyCache: true,
	}
}

// Returns the result of 'dmidecode -s <key>'
func String(key string) string {
	return cache.str(key)
}
func (d dmiCache) str(key string) string {
	str, ok := d.strings[key]
	if ok || d.onlyCache {
		return str
	}
	dmiCmd := exec.Command("dmidecode", "-s", key)
	out, err := dmiCmd.CombinedOutput()
	if err != nil {
		log.Logf("error %s executing %v\noutput:%s\n", err, dmiCmd.Args, out)
		return ""
	}
	if len(out) > 3 {
		e := bytes.LastIndex(out, []byte("\n"))
		nl := bytes.LastIndex(out[:e], []byte("\n"))
		//if dmidecode doesn't like the data presented, it may print a second line with an error
		if bytes.HasPrefix(out[nl+1:e], []byte("Invalid entry")) {
			e = nl
			nl = bytes.LastIndex(out[:e], []byte("\n"))
		}
		out = out[nl+1 : e]
	}
	str = strings.TrimSpace(string(out))
	d.strings[key] = str
	return str
}

//Return all entries for a given dmi type, same format as produced by 'dmidecode -t <n>'.
func Entries(dmiType int) []byte {
	return cache.entries(dmiType)
}
func (d dmiCache) entries(dmiType int) []byte {
	if d.onlyCache && d.types == nil {
		return nil
	}
	out, ok := d.types[dmiType]
	if ok || d.onlyCache {
		return out
	}
	dmiCmd := exec.Command("dmidecode", "-t", fmt.Sprintf("%d", dmiType))
	out, err := dmiCmd.CombinedOutput()
	if err != nil {
		log.Logf("error %s reading DMI field %d\noutput:%s\n", err, dmiType, out)
		return nil
	}
	return out
}

//return data for line matching fieldName in given struct type
//fieldName must be all non-whitespace chars at beginning of desired line, i.e. "SKU Number:".
//Using "SKU" would result in the returned string beginning with "Number: "
func Field(dmiType int, fieldName string) (field string) {
	return cache.field(dmiType, fieldName)
}
func (d dmiCache) field(dmiType int, fieldName string) (field string) {
	return d.fieldN(dmiType, 0, fieldName)
}

//Nth entry for a particular type, with N starting at 0
func FieldN(dmiType, entry int, fieldName string) (field string) {
	return cache.fieldN(dmiType, entry, fieldName)
}
func (d dmiCache) fieldN(dmiType, entry int, fieldName string) (field string) {
	out := d.entries(dmiType)
	buf := bytes.NewBuffer(out)
	var line string
	var err error
	currentEntry := -1
	for {
		line, err = buf.ReadString('\n')
		if err != nil && err != io.EOF {
			log.Logf("error %s reading DMI field %d'%s' from dmidecode output - got %s\n", err, dmiType, fieldName, line)
		}
		if strings.HasPrefix(line, "Handle 0x") {
			currentEntry++
			continue
		}
		wsTrim := strings.TrimSpace(line)
		pfxTrim := strings.TrimPrefix(wsTrim, fieldName)
		//pfxTrim and wsTrim will be identical if the specified prefix didn't exist
		if pfxTrim != wsTrim && currentEntry == entry {
			field = strings.TrimSpace(pfxTrim)
			return
		}
		if err != nil {
			break
		}
	}

	log.Logf("error, no data for DMI field '%d[%d] %s'; err=%s\noutput:%s\n", dmiType, entry, fieldName, err, out)
	return
}

func Clear() {
	cache.strings = make(DmiStrMap)
	cache.types = make(DmiTypeMap)
	cache.onlyCache = false
}

// Run dmidecode to dump all entries found. Not cached.
func Dump() {
	dmiCmd := exec.Command("dmidecode")
	out, err := dmiCmd.CombinedOutput()
	if err != nil {
		log.Logf("error executing dmidecode: %s", err)
	}
	log.Logf("------ DMI raw output ------\n%s\n", string(out))
}
