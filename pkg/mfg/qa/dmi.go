// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package qa

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/purecloudlabs/gprovision/pkg/hw/dmi"
	"github.com/purecloudlabs/gprovision/pkg/hw/ipmi"
	"github.com/purecloudlabs/gprovision/pkg/log"
	"github.com/purecloudlabs/gprovision/pkg/mfg/mfgflags"
)

type DmiMap map[string]string

//what about dmi types with multiple entries, such as type 17 (DIMM)?
//compare against each entry and ensure all match??

func (matches DmiMap) Populate() {
	//if we encounter an error we continue, under the assumption that the data we do set won't match the desired data
	for k := range matches {
		if !strings.Contains(k, " ") {
			matches[k] = dmi.String(k)
		} else {
			s := strings.SplitN(k, " ", 2)
			if len(s) != 2 {
				log.Logf("Bad entry '%s' in %v", k, matches)
				continue
			}
			dmiEntry := 0
			if strings.Contains(s[0], "[") {
				t := strings.Split(s[0], "[")
				if len(t) != 2 {
					log.Logf("Bad entry '%s' in %v", k, matches)
					continue
				}
				s[0] = t[0] //so code below can get dmiType

				dmiEntryStr := strings.Trim(t[1], "]")
				var err error
				dmiEntry, err = strconv.Atoi(dmiEntryStr)
				if err != nil {
					log.Logf("dmiEntry Atoi(%s): %s", dmiEntryStr, err)
					continue
				}
			}
			dmiType, err := strconv.Atoi(s[0])
			if err != nil {
				log.Logf("dmiType Atoi(%s): %s", s[0], err)
				continue
			}
			if !strings.Contains(s[1], ":") {
				s[1] += ":"
			}
			matches[k] = dmi.FieldN(dmiType, dmiEntry, s[1])
		}
	}
}

func (required DmiMap) Compare(detected DmiMap) (errors int) {
	for k, v := range required {
		if len(k) > 1 && k[0] == '_' {
			//keys beginning with _ are treated as comments
			continue
		}
		if detected[k] != v {
			errors += 1
			log.Msgf("DMI mismatch for %s", k)
			log.Logf("DMI '%s': want %s, got %s", k, v, detected[k])
		}
	}
	if errors == 0 {
		log.Msg("+++ DMI data: match +++")
	} else {
		log.Msgf("!!! DMI data: %d errors !!!", errors)
	}
	return
}

func dumpDMIData(alwaysRaw bool) {
	dmiStrings := []string{
		"bios-vendor", "bios-version", "bios-release-date",
		"system-manufacturer", "system-product-name", "system-version", "system-serial-number", "system-uuid",
		"baseboard-manufacturer", "baseboard-product-name", "baseboard-version", "baseboard-serial-number", "baseboard-asset-tag",
		"chassis-manufacturer", "chassis-type", "chassis-version", "chassis-serial-number", "chassis-asset-tag",
		"processor-family", "processor-manufacturer", "processor-version", "processor-frequency"}
	var out string
	for _, s := range dmiStrings {
		out += fmt.Sprintf("%25s='%s'\n", s, dmi.String(s))
	}
	log.Logf("------ DMI string dump ------\n%s\n", out)
	if alwaysRaw || mfgflags.Flag(mfgflags.RawDmi) {
		dmi.Dump() //logs dmidecode raw output
	}
}

type FirmwareVer struct {
	BIOS string `json:",omitempty"`
	IPMI string `json:",omitempty"`
	FRU  string `json:",omitempty"`
	SDR  string `json:",omitempty"`
	ME   string `json:",omitempty"`
}

func (f *FirmwareVer) Populate() {
	f.BIOS = dmi.String("bios-version")
	if ipmi.Available() {
		f.IPMI, f.FRU, f.SDR, f.ME = ipmi.Versions()
	} else {
		log.Logf("IPMI info not available")
	}
}

func (required FirmwareVer) Compare(detected FirmwareVer) (errors int) {
	if required.BIOS != "" && required.BIOS != detected.BIOS {
		errors += 1
		log.Msg("!!! version mismatch for BIOS !!!")
	}
	if required.IPMI != "" && required.IPMI != detected.IPMI {
		errors += 1
		log.Msg("!!! version mismatch for IPMI !!!")
	}
	if required.FRU != "" && required.FRU != detected.FRU {
		errors += 1
		log.Msg("!!! version mismatch for FRU !!!")
	}
	if required.SDR != "" && required.SDR != detected.SDR {
		errors += 1
		log.Msg("!!! version mismatch for SDR !!!")
	}
	if required.ME != "" && required.ME != detected.ME {
		errors += 1
		log.Msg("!!! version mismatch for ME !!!")
	}
	if errors == 0 {
		log.Msg("+++ Firmware: match +++")
	}
	return errors
}
