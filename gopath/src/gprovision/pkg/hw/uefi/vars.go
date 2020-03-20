// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package uefi

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"gprovision/pkg/log"
	"io/ioutil"
	"os"
	fp "path/filepath"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

//http://kurtqiao.github.io/uefi/2015/01/13/uefi-boot-manager.html

var efiVarDir = "/sys/firmware/efi/vars"

const (
	bootUuid = "8be4df61-93ca-11d2-aa0d-00e098032b8c"
)

//a generic efi var
type EfiVar struct {
	uuid, name string
	data       []byte
}
type EfiVars []EfiVar

func ReadVar(uuid, name string) (e EfiVar, err error) {
	path := fp.Join(efiVarDir, name+"-"+uuid, "data")
	e.uuid = uuid
	e.name = name
	e.data, err = ioutil.ReadFile(path)
	return
}
func AllVars() (vars EfiVars) {
	entries, err := fp.Glob(fp.Join(efiVarDir, "*-*"))
	if err != nil {
		log.Logf("error reading efi vars: %s", err)
		return
	}
	for _, entry := range entries {
		info, err := os.Stat(entry)
		if err == nil && info.IsDir() {
			components := strings.SplitN(fp.Base(info.Name()), "-", 2)
			if len(components) != 2 {
				log.Logf("skipping %s - not a valid var?", info.Name())
				continue
			}
			v, err := ReadVar(components[1], components[0])
			if err != nil {
				log.Logf("reading efi var %s: %s", info.Name(), err)
				continue
			}
			vars = append(vars, v)
		}
	}
	return
}

//a boot entry; will have the name BootXXXX where XXXX is hexadecimal
type BootEntryVar struct {
	Number             uint16
	Attributes         uint32
	FilePathListLength uint16
	Description        string
	FilePathList       []byte
	OptionalData       []byte
}
type BootEntryVars []*BootEntryVar

func ReadBootVar(num uint16) (b *BootEntryVar) {
	v, err := ReadVar(bootUuid, fmt.Sprintf("Boot%04X", num))
	if err != nil {
		log.Logf("reading var Boot%04X: %s", num, err)
		return
	}
	return v.BootVar()
}

//decodes an efivar as a boot entry. use IsBootEntry() to screen first.
func (v EfiVar) BootVar() (b *BootEntryVar) {
	num, err := strconv.ParseUint(v.name[4:], 16, 16)
	if err != nil {
		log.Logf("error parsing boot var %s: %s", v.name, err)
	}
	b = new(BootEntryVar)
	b.Number = uint16(num)
	b.Attributes = binary.LittleEndian.Uint32(v.data[:4])
	b.FilePathListLength = binary.LittleEndian.Uint16(v.data[4:6])

	//Description is null-terminated utf16
	var i uint16
	for i = 6; ; i += 2 {
		if v.data[i] == 0 {
			break
		}
	}
	b.Description, err = DecodeUTF16(v.data[6:i])
	if err != nil {
		log.Logf("reading description: %s (%d -> %x)", err, i, v.data[6:i])
	}

	b.FilePathList = v.data[i+2 : i+2+b.FilePathListLength]
	b.OptionalData = v.data[i+2+b.FilePathListLength:]
	return
}

func (b BootEntryVar) String() string {
	opts, err := DecodeUTF16(b.OptionalData)
	if err != nil {
		opts = string(b.OptionalData)
	}
	//could decode uefi path... worth it?
	return fmt.Sprintf("Boot%04X: attrs=0x%x, desc=%s, path=%x, opts=%q", b.Number, b.Attributes, b.Description, b.FilePathList, opts)
}

func (b BootEntryVar) Remove() error {
	return RemoveBootEntry(b.Number)
}

//returns list of boot entries (BootXXXX)
//note that BootCurrent, BootOptionSupport, BootNext, BootOrder, etc do not count as boot entries.
func AllBootEntryVars() BootEntryVars {
	return AllVars().BootEntries()
}

//from a list of efi vars, parse any that are boot entries and return list of them
func (vars EfiVars) BootEntries() (bootvars BootEntryVars) {
	for _, v := range vars {
		if v.IsBootEntry() {
			bootvars = append(bootvars, v.BootVar())
		}
	}
	return
}

func (e EfiVar) IsBootEntry() bool {
	if e.uuid != bootUuid || len(e.name) != 8 || e.name[:4] != "Boot" {
		return false
	}
	_, err := strconv.ParseUint(e.name[4:], 16, 16)
	return err == nil
}

//filter list of boot entries to exclude entries we didn't create
func (entries BootEntryVars) Ours() (bootvars BootEntryVars) {
	for _, b := range entries {
		if b.IsOurs() {
			bootvars = append(bootvars, b)
		}
	}
	return
}
func (b BootEntryVar) IsOurs() bool {
	switch BootLabel(b.Description) {
	case BootLabelFR:
		return true
	case BootLabelNorm:
		return true
	case BootLabelErase:
		return true
	}
	return false
}

//https://gist.github.com/bradleypeabody/185b1d7ed6c0c2ab6cec
func DecodeUTF16(b []byte) (string, error) {
	if len(b)%2 != 0 {
		return "", fmt.Errorf("Must have even length byte slice")
	}

	u16s := make([]uint16, 1)
	ret := &bytes.Buffer{}
	b8buf := make([]byte, 4)

	lb := len(b)
	for i := 0; i < lb; i += 2 {
		u16s[0] = uint16(b[i]) + (uint16(b[i+1]) << 8)
		r := utf16.Decode(u16s)
		n := utf8.EncodeRune(b8buf, r[0])
		ret.Write(b8buf[:n])
	}

	return ret.String(), nil
}
