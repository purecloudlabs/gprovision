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

//Returns all efi variables
func AllVars() (vars EfiVars) { return ReadVars(nil) }

//Returns efi variables matching filter
func ReadVars(filt VarFilter) (vars EfiVars) {
	entries, err := fp.Glob(fp.Join(efiVarDir, "*-*"))
	if err != nil {
		log.Logf("error reading efi vars: %s", err)
		return
	}
	for _, entry := range entries {
		base := fp.Base(entry)
		n := strings.Count(base, "-")
		if n < 5 {
			log.Logf("skipping %s - not a valid var?", base)
			continue
		}
		components := strings.SplitN(base, "-", 2)
		if filt != nil && !filt(components[1], components[0]) {
			continue
		}
		info, err := os.Stat(entry)
		if err == nil && info.IsDir() {
			v, err := ReadVar(components[1], components[0])
			if err != nil {
				log.Logf("reading efi var %s: %s", base, err)
				continue
			}
			vars = append(vars, v)
		}
	}
	return
}

//A boot entry. Will have the name BootXXXX where XXXX is hexadecimal
type BootEntryVar struct {
	Number uint16 //from the var name
	EfiLoadOption
}

/* EfiLoadOption defines the data struct used for vars such as BootXXXX.
As defined in UEFI spec v2.8A:
    typedef struct _EFI_LOAD_OPTION {
        UINT32 Attributes;
        UINT16 FilePathListLength;
        // CHAR16 Description[];
        // EFI_DEVICE_PATH_PROTOCOL FilePathList[];
        // UINT8 OptionalData[];
    } EFI_LOAD_OPTION;
*/
type EfiLoadOption struct {
	Attributes         uint32
	FilePathListLength uint16
	Description        string
	FilePathList       EfiDevicePathProtocolList
	OptionalData       []byte
}
type BootEntryVars []*BootEntryVar

// Gets BootXXXX var, if it exists
func ReadBootVar(num uint16) (b *BootEntryVar) {
	v, err := ReadVar(bootUuid, fmt.Sprintf("Boot%04X", num))
	if err != nil {
		log.Logf("reading var Boot%04X: %s", num, err)
		return
	}
	return v.BootVar()
}

// Reads BootCurrent, and from there gets the BootXXXX var referenced.
func ReadCurrentBootVar() (b *BootEntryVar) {
	curr := ReadBootCurrent()
	if curr == nil {
		return nil
	}
	return ReadBootVar(curr.Current)
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
	b.OptionalData = v.data[i+2+b.FilePathListLength:]

	b.FilePathList, err = ParseFilePathList(v.data[i+2 : i+2+b.FilePathListLength])
	if err != nil {
		log.Logf("parsing FilePathList in %s: %s", b.String(), err)
	}
	return
}

func (b BootEntryVar) String() string {
	opts, err := DecodeUTF16(b.OptionalData)
	if err != nil {
		opts = string(b.OptionalData)
	}
	//could decode uefi path... worth it?
	return fmt.Sprintf("Boot%04X: attrs=0x%x, desc=%q, path=%s, opts=%x", b.Number, b.Attributes, b.Description, b.FilePathList.String(), opts)
}

func (b BootEntryVar) Remove() error {
	return RemoveBootEntry(b.Number)
}

//returns list of boot entries (BootXXXX)
//note that BootCurrent, BootOptionSupport, BootNext, BootOrder, etc do not count as boot entries.
func AllBootEntryVars() BootEntryVars {
	//return AllVars().BootEntries()
	//BootEntries() is somewhat redundant, but parses the vars into BootEntryVar{}
	return ReadVars(BootEntryFilter).BootEntries()
}

//Return all boot-related uefi vars
func AllBootVars() EfiVars {
	return ReadVars(BootVarFilter)
}

//A type of function used to filter efi vars
type VarFilter func(uuid, name string) bool

// A VarNameFilter passing boot-related vars. These are a superset of those
// returned by BootEntryFilter.
func BootVarFilter(uuid, name string) bool {
	return uuid == bootUuid && strings.HasPrefix(name, "Boot")
}

// A VarNameFilter passing boot entries. These are a subset of the vars
// returned by BootVarFilter.
func BootEntryFilter(uuid, name string) bool {
	if !BootVarFilter(uuid, name) {
		return false
	}
	// Boot entries begin with BootXXXX-, where XXXX is hex.
	//First, check for the dash.
	if len(name) != 8 {
		return false
	}
	// Try to parse XXXX as hex. If it parses, it's a boot entry.
	_, err := strconv.ParseUint(name[4:], 16, 16)
	return err == nil
}

//Returns a filter negating the given filter.
func NotFilter(f VarFilter) VarFilter {
	return func(u, n string) bool { return !f(u, n) }
}

//Returns true only if all given filters return true.
func AndFilter(filters ...VarFilter) VarFilter {
	return func(u, n string) bool {
		for _, f := range filters {
			if !f(u, n) {
				return false
			}
		}
		return true
	}
}

func (vars EfiVars) Filter(filt VarFilter) EfiVars {
	var res EfiVars
	for _, v := range vars {
		if filt(v.uuid, v.name) {
			res = append(res, v)
		}
	}
	return res
}

type BootCurrentVar struct {
	EfiVar
	Current uint16
}

//returns the BootCurrent var
func (vars EfiVars) BootCurrent() *BootCurrentVar {
	for _, v := range vars {
		if v.name == "BootCurrent" {
			return &BootCurrentVar{
				EfiVar:  v,
				Current: uint16(v.data[1])<<8 | uint16(v.data[0]),
			}
		}
	}
	return nil
}

func ReadBootCurrent() *BootCurrentVar {
	v, err := ReadVar(bootUuid, "BootCurrent")
	if err != nil {
		log.Logf("reading uefi BootCurrent var: %s", err)
		return nil
	}
	return &BootCurrentVar{
		EfiVar:  v,
		Current: uint16(v.data[1])<<8 | uint16(v.data[0]),
	}
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
		u16s[0] = bytesToU16(b[i : i+2])
		r := utf16.Decode(u16s)
		n := utf8.EncodeRune(b8buf, r[0])
		ret.Write(b8buf[:n])
	}

	return ret.String(), nil
}

func bytesToU16(b []byte) uint16 {
	if len(b) != 2 {
		log.Fatalf("bytesToU16: bad len %d (%x)", len(b), b)
	}
	return uint16(b[0]) + (uint16(b[1]) << 8)
}
