// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

//Package QA implements quality checks for the mfg process. In the event of
//success, it can print out a QA report. It can also log voluminous hardware
//details.
package qa

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"regexp"
	"strconv"
	"strings"

	"github.com/purecloudlabs/gprovision/pkg/appliance"
	"github.com/purecloudlabs/gprovision/pkg/common"
	"github.com/purecloudlabs/gprovision/pkg/log"
	"github.com/purecloudlabs/gprovision/pkg/mfg/mfgflags"
)

// TODO document process of adding a new platform

// TODO  IPMI

type RamMegs uint64
type Devices struct {
	PCI PciDevices `json:",omitempty"`
	USB UsbDevices `json:",omitempty"`
}
type Specs struct {
	//Must match a name in appliance data.
	DevCodeName string

	CPUInfo CPUInfo
	RamMegs RamMegs

	Recovery        RecoveryDisk
	MainDiskConfigs MainDiskConfigs //from json
	chosenDiskCfg   int             //index into MainDiskConfigs
	mainDisks       MainDisks       //from hw detection

	//Number of NICs that must have specific prefix. See also: strs.NicPrefix()
	NumOUINics int
	//True if there must be no gaps between MACs on NICs matching prefix.
	OUINicsSequential bool
	//Total NICs, with and without prefix.
	TotalNics int

	FirmwareVer FirmwareVer `json:",omitempty"`
	//IPMI info?

	Devices Devices `json:",omitempty"`

	SerNumRegex string `json:",omitempty"`
	/* list dmidecode named fields:
	for f in $(dmidecode -s |& grep '^  '); do
	  printf "%25s : %s\n" "$f" "$(sudo dmidecode -s $f)"
	done
	*/
	DmiMatches DmiMap //support named fields _and_ anything else reported
}

func (s Specs) String() string {
	format := `
	Codename: %s
	CPUInfo: %s
	RamMegs: %d
	Recovery: %s
	MainDisks: %v
	NumOUINics: %d
	OUINicsSequential: %t
	TotalNics: %d
	FirmwareVer: %#v
	PCI Devices: %v
	USB Devices: %v
	SerNumRegex: %s
	DmiMatches: %#v
	`
	return fmt.Sprintf(format, s.DevCodeName, s.CPUInfo, s.RamMegs, Disk(s.Recovery),
		s.mainDisks, s.NumOUINics, s.OUINicsSequential, s.TotalNics,
		s.FirmwareVer, s.Devices.PCI, s.Devices.USB, s.SerNumRegex, s.DmiMatches)
}

//Dump as much data as possible, including raw dmi output. Used for unrecognized platforms.
func Dump() {
	var required, detected Specs
	detected.Populate(nil)
	dump(required, detected, true)
}

//Dump hardware details. Used for new platforms where params are unknown, and for existing platforms with bad/missing hardware.
func dump(required, detected Specs, alwaysRawDMI bool) {
	//fiddle with the data a bit to make it less confusing for anyone looking at the output
	detected.Recovery = RecoveryDisk{
		Vendor: "*** Don't know what's what; recovery disk will be lumped in with main disks below ***",
	}
	found := FoundDisks()
	md := MainDisks{}
	for _, f := range found {
		md = append(md, (*MainDisk)(f))
	}
	detected.MainDiskConfigs = MainDiskConfigs{md}

	if mfgflags.Verbose {
		log.Logf("required:\n%s\n\ndetected:\n%s", required, detected)
	}
	m, e := json.MarshalIndent(detected, "  ", "  ")
	if e != nil {
		log.Logf("cannot output json, marshalling error: %s", e)
	} else {
		log.Logf("==== detected data in json format follows ====\n%s\n==== end of json ====", string(m))
	}
	dumpDMIData(alwaysRawDMI)
}

/* Validate that the current device meets the specs for its type
   This occurs in 3 stages.
   * initialization: fill in detected specs structure with basic info about some things (e.g. identifiers for pci devices we care about)
   * population: write details of hardware that exists on this model to detected struct
   * validation: compare detected and required specs
*/
func (required Specs) Validate(platform common.PlatInfoer) {
	required.SanityCheck()
	checkSN(platform.SerNum(), required.SerNumRegex)

	detected := required.InitDetected()
	detected.Populate(platform)
	err := required.Compare(detected)
	if err != nil {
		dump(required, detected, false)
		log.Fatalf("detected specs for %s do not match required specs: %s", platform.DeviceCodeName(), err)
	}
}

//Check serial number. Ignores failure if unit is a prototype, identified via PROTO_IDENT.
func checkSN(sn, re string) {
	//MustCompile would panic, meaning the error would only show up on an attached display
	r, err := regexp.Compile(re)
	if err != nil {
		log.Fatalf("error in SerNum regex (%q): %s", re, err)
	}
	if !r.MatchString(sn) {
		msg := fmt.Sprintf("serial number %s does not match %s", sn, re)
		if appliance.IdentifiedViaFallback() {
			log.Log(msg + "; ignoring - identified via fallback")
		} else {
			log.Fatalf(msg)
		}
	}
}

//these checks just make sure the json didn't have a typo that caused something to not be filled in
//far from perfect, but should catch _some_ problems
func (r Specs) SanityCheck() {
	var totalDiskCfgs, emptyDiskCfgs int
	for _, c := range r.MainDiskConfigs {
		if len(c) == 0 {
			emptyDiskCfgs++
		} else {
			totalDiskCfgs++
		}
	}
	if emptyDiskCfgs > 0 || totalDiskCfgs == 0 {
		log.Fatalf("sanity check: disk config is wrong for %s (typo in json?)", r.DevCodeName)
	}
	if r.Recovery.Size < 1073741824 {
		//arbitrary 1G lower limit. actual smallest usable size will be larger.
		log.Fatalf("sanity check: recovery disk spec is bad (typo in json?)")
	}
}

func (required Specs) Compare(detected Specs) (err error) {
	errors := 0
	errors += logNE(required.CPUInfo, detected.CPUInfo, "CPU Info")

	errors += logNE(required.NumOUINics, detected.NumOUINics, "Number of OUI-prefixed NICs")
	errors += logNE(required.OUINicsSequential, detected.OUINicsSequential, "OUI-prefixed NICs sequential")
	errors += logNE(required.TotalNics, detected.TotalNics, "Total NICs")

	errors += required.RamMegs.Compare(detected.RamMegs)
	errors += required.FirmwareVer.Compare(detected.FirmwareVer)

	errCount := errors //if there are errors related to recovery or main disks, dump disk info to log
	errors += required.Recovery.Compare(detected.Recovery)
	errors += required.MainDiskConfigs.Compare(detected.mainDisks, detected.chosenDiskCfg)
	if errors != errCount {
		DumpDisks()
	}

	errors += required.Devices.USB.Compare(detected.Devices.USB)
	errors += required.Devices.PCI.Compare(detected.Devices.PCI)

	errors += required.DmiMatches.Compare(detected.DmiMatches)

	if errors != 0 {
		err = fmt.Errorf("%d problems detected", errors)
	} else {
		log.Msg("hw validation: no problems")
	}
	return err
}

//compare two items, log any mismatch. return 1 if error found.
func logNE(required, detected interface{}, desc string) int {
	if required != detected {
		log.Msgf("!!! Mismatch in %s !!!", desc)
		log.Logf("%s: got %v, want %v", desc, detected, required)
		return 1
	}
	log.Msgf("+++ %s: match +++", desc)
	return 0
}

func (s *Specs) Populate(platform common.PlatInfoer) {
	if platform != nil {
		s.DevCodeName = platform.DeviceCodeName()
	}
	s.CPUInfo.Read()
	s.RamMegs = getRam()

	s.GetNicInfo()

	s.Recovery.Populate()
	s.mainDisks, s.chosenDiskCfg = PopulateDisks(s.MainDiskConfigs)
	s.Devices.PCI.Populate()
	s.Devices.USB.Populate()
	s.DmiMatches.Populate()
	s.FirmwareVer.Populate()
}

//copy some info from required specs to detected, so Populate funcs know what hardware is of interest
func (r Specs) InitDetected() (d Specs) {
	d.Recovery = r.Recovery
	d.Recovery.Size = 0
	d.MainDiskConfigs = r.initDisks()
	d.DmiMatches = make(DmiMap)
	for k := range r.DmiMatches {
		if len(k) > 1 && k[0] == '_' {
			//keys beginning with _ are treated as comments
			continue
		}
		d.DmiMatches[k] = ""
	}
	return
}

func (r Specs) initDisks() (cfgs MainDiskConfigs) {
	numDisks := len(r.MainDiskConfigs[0])
	for i := range r.MainDiskConfigs {
		if len(r.MainDiskConfigs[i]) != numDisks {
			//while mfg app could support a change in disk quantity without too much trouble, factory restore can't
			log.Fatalf("MainDisks alternatives must all have the same number of disks; difference between index 0 and %d: %#v", i, r.MainDiskConfigs)
		}
	}
	for _, s := range r.MainDiskConfigs {
		var st MainDisks
		for _, m := range s {
			var n = new(MainDisk)
			*n = *m
			n.Size = 0
			st = append(st, n)
		}
		cfgs = append(cfgs, st)
	}
	return
}

func (req RamMegs) Compare(det RamMegs) (errors int) {
	inTol := float64(req)*1.01 > float64(det) && float64(req)*.99 < float64(det)
	if !inTol {
		errors = 1
		log.Msg("!!! Mismatch in memory !!!")
		log.Logf("Memory: got %d, want %d", det, req)
	} else {
		log.Msg("+++ Memory: match +++")
	}
	return
}

func getRam() (megs RamMegs) {
	d, err := ioutil.ReadFile("/proc/meminfo")
	if err != nil {
		log.Fatalf("error reading memory info: %s", err)
	}
	buf := bytes.NewBuffer(d)
	for {
		l, err := buf.ReadString('\n')

		if err != nil {
			log.Fatalf("error detecting system RAM: %s", err)
		}
		if len(l) == 0 {
			break
		}
		if strings.HasPrefix(l, "MemTotal:") {
			//MemTotal:       16377520 kB
			s1 := strings.Split(l, ":")
			if len(s1) < 2 {
				break
			}
			s2 := strings.Split(strings.TrimSpace(s1[1]), " ")
			if len(s2) < 1 {
				break
			}
			kb, err := strconv.ParseUint(s2[0], 10, 64)
			if err != nil {
				log.Fatalf("error detecting system RAM: %s", err)
			}
			megs = RamMegs(kb / 1024)
			break
		}
	}
	return
}
