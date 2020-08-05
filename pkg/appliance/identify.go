// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

// +build !light

package appliance

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/purecloudlabs/gprovision/pkg/hw/block"
	"github.com/purecloudlabs/gprovision/pkg/hw/dmi"
	"github.com/purecloudlabs/gprovision/pkg/log"
)

// cause bindata.go to be generated from files in the data dir
//go:generate ../../bin/go-bindata -tags !light -prefix=../../proprietary/data/appliance -pkg=$GOPACKAGE ../../proprietary/data/appliance

func init() {
	if strings.Contains(os.Args[0], ".test") {
		//compiled/executed by 'go test', so don't load json
		return
	}
	//get either the generic 'aj_default' or the go-bindata version
	j := getJson()
	err := loadJson(j)
	if err != nil {
		log.Logf("loading default json: %s", err)
		log.Fatalf("json error")
	}
}

//TODO move to per-variant match function?
//TODO add bool to Variant to indicate whether to use baseboard-* or system-*
func Identify() *Variant {
	if identifiedVariant != nil {
		return identifiedVariant
	}
	// mfg/sysMfg and prod/sysProd are set by the oem and typically aren't very specific
	// sku is more specific and at times is set by us
	mfg := dmi.String("baseboard-manufacturer")
	prod := dmi.String("baseboard-product-name")
	sku := dmi.Field(1, "SKU Number:") //this is the default field for SKU, may be overridden for paricular models

	for _, v := range variants {
		if mfg == v.DmiMbMfg && prod == v.DmiProdName {
			overrideSku, success := v.checkModelRe(mfg, prod, sku)
			if !success {
				continue
			}
			if !v.processorMatch() {
				continue
			}
			identifiedVariant = &Variant{
				i:      v,
				mfg:    mfg,
				prod:   prod,
				sku:    overrideSku,
				serial: dmi.String(v.SerNumField),
			}
			return identifiedVariant
		}
	}
	log.Logf("attempting alternate identification method...")
	sysMfg := dmi.String("system-manufacturer")
	sysProd := dmi.String("system-product-name")
	for _, v := range variants {
		if sysMfg == v.DmiMbMfg && sysProd == v.DmiProdName {
			overrideSku, success := v.checkModelRe(sysMfg, sysProd, sku)
			if !success {
				continue
			}
			if !v.processorMatch() {
				continue
			}
			identifiedVariant = &Variant{
				i:      v,
				mfg:    sysMfg,
				prod:   sysProd,
				sku:    overrideSku,
				serial: dmi.String(v.SerNumField),
			}
			return identifiedVariant
		}
	}
	log.Logf("no match for mfg|prod %s | %s (or  %s | %s ), CPU %q\nknown platforms:\n", mfg, prod, sysMfg, sysProd, dmi.String("processor-version"))
	for _, v := range variants {
		log.Log(v.DiagSummary())
	}
	return nil
}

// Check that the unit's cpus match what the system reports.
func (v *Variant_) processorMatch() bool {
	if len(v.CPU) == 0 {
		return true
	}
	//system with 1 populated socket, 1 empty:
	//Cpu 1\nNot Specified
	//not sure if "Not Specified" comes from dmidecode or is something the vendor chooses
	current := strings.TrimSpace(dmi.String("processor-version"))

	//not sure the cpus will always be in the same order, so loop over them
	vcpus := strings.Split(v.CPU, "\n")
	ccpus := strings.Split(current, "\n")
	for _, vc := range vcpus {
		for i := range ccpus {
			cc := strings.TrimSpace(ccpus[i])
			if cc == vc {
				ccpus = append(ccpus[:i], ccpus[i+1:]...)
				break
			}
		}
	}
	//if it matches, ccpus will be empty
	return len(ccpus) == 0
}

func ReIdentify() *Variant {
	identifiedVariant = nil
	return Identify()
}

func (v *Variant_) checkModelRe(mfg, prod, sku string) (overrideSku string, match bool) {
	re, err := regexp.Compile(v.DmiProdModelRegex)
	if err != nil {
		log.Logf("skipping potential match %s:%s, as regex %s failed to compile - err=%s",
			mfg, prod, v.DmiProdModelRegex, err)
		return "", false
	}
	if v.DmiPMOverrideFld != "" {
		sku = dmi.Field(v.DmiPMOverrideTyp, v.DmiPMOverrideFld)
	}
	if !re.Match([]byte(sku)) {
		log.Logf("skipping potential match %s:%s, as regex %s failed to match %s",
			mfg, prod, v.DmiProdModelRegex, sku)
		return "", false
	}
	log.Logf("+++ device matches %s - %s:%s:%s +++", v.DevCodeName, mfg, prod, sku)
	return sku, true
}

//one-line summary
func (v *Variant_) DiagSummary() string {
	s := fmt.Sprintf("  %s | %s | SKU %s   (%s)", v.DmiMbMfg, v.DmiProdName, v.DmiProdModelRegex, v.DevCodeName)
	if v.CPU != "" {
		s += fmt.Sprintf(", CPU=%q", v.CPU)
	}
	return s
}

//returns names of block devices which could be the recovery device
//excludes virtual devices and ones with incorrect size
func (v *Variant) RecoveryCandidates(RecoverySize uint64) (devList []string) {
	for _, dev := range block.Devices() {
		if dev.Size == RecoverySize {
			log.Logf("recovery candidate: %s (%d)", dev.Name, dev.Size)
			devList = append(devList, dev.Name)
		} else {
			log.Logf("rejected recovery candidate: %s (%d != %d)", dev.Name, dev.Size, RecoverySize)
		}
	}
	return
}

// Call Identify(); if that fails, use fallback() to get the stored code name.
// Use appliance.json section matching that codename. The fallback() func
// probably gets the codename from the file /platform on the recovery volume
// (and ultimately from the windows disktag).
func IdentifyWithFallback(fallback func() (string, error)) (*Variant, error) {
	Identify()
	if identifiedVariant != nil {
		return identifiedVariant, nil
	}
	ident, err := fallback()
	if err != nil {
		return nil, err
	}
	usedFallback = true
	for _, v := range variants {
		var sku string
		if v.DevCodeName == ident {
			if v.DmiPMOverrideFld != "" {
				sku = dmi.Field(v.DmiPMOverrideTyp, v.DmiPMOverrideFld)
			} else {
				sku = dmi.Field(1, "SKU Number:")
			}

			identifiedVariant = &Variant{
				i:      v,
				mfg:    v.DmiMbMfg,
				prod:   v.DmiProdName,
				sku:    sku,
				serial: dmi.String(v.SerNumField),
			}
			break
		}
	}
	return identifiedVariant, nil
}

var usedFallback bool

func IdentifiedViaFallback() bool {
	return usedFallback
}

func Get(codename string) *Variant {
	for _, v := range variants {
		if v.DevCodeName == codename {
			return &Variant{i: v}
		}
	}
	return nil
}

var aj_default = `{"Variants":[
{"Familyname":"qemu","DmiMbMfg":"GPROV_QEMU","DmiProdName":"9p2k_dev","DmiProdModelRegex":".*","SerNumField":"system-serial-number","NumDataDisks":1,"Disksize":5368709120,"DiskIsSSD":true,"SwRaidlevel":-1,"Virttype":2,"NICInfo":{"SharedDiagPorts":[],"WANIndex":0,"DefaultNamesNoDiag":["Port 1 (WAN)"]},"RecoveryMedia":{"LocateRDMethod":"9p","ValidateRDMethod":"9p","FsType":"9p","FsOptsAdditional":"trans=virtio,version=9p2000.L"},"Lcd":"none","DevCodeName":"QEMU","Prototype":true},
{"Familyname":"qemu","DmiMbMfg":"GPROV_QEMU","DmiProdName":"mfg_test","DmiProdModelRegex":".*","SerNumField":"system-serial-number","NumDataDisks":1,"Disksize":21474836480,"DiskIsSSD":true,"SwRaidlevel":-1,"Virttype":2,"NICInfo":{"SharedDiagPorts":[],"WANIndex":0,"DefaultNamesNoDiag":["Port 1 (WAN)"]},"RecoveryMedia":{"LocateRDMethod":"byLabel","ValidateRDMethod":"usb","FsType":"ext3","SSD":true},"Lcd":"none","DevCodeName":"QEMU-mfg-test","Prototype":true},
{"_comment":"identical to the above, except for prod name & ipmi being enabled",
 "Familyname":"qemu","DmiMbMfg":"GPROV_QEMU","DmiProdName":"mfg_test_ipmi","DmiProdModelRegex":".*","SerNumField":"system-serial-number","NumDataDisks":1,"Disksize":21474836480,"DiskIsSSD":true,"SwRaidlevel":-1,"Virttype":2,"NICInfo":{"SharedDiagPorts":[],"WANIndex":0,"DefaultNamesNoDiag":["Port 1 (WAN)"]},"IPMI":true,"RecoveryMedia":{"LocateRDMethod":"byLabel","ValidateRDMethod":"usb","FsType":"ext3","SSD":true},"Lcd":"none","DevCodeName":"QEMU-mfg-test","Prototype":true},
{"Familyname":"cputest","DmiMbMfg":"cputest","DmiProdName":"cputest","DmiProdModelRegex":".*","CPU":"someVersion","SerNumField":"system-serial-number","NumDataDisks":1,"Disksize":21474836480,"DiskIsSSD":true,"SwRaidlevel":-1,"Virttype":2,"NICInfo":{"SharedDiagPorts":[],"WANIndex":0,"DefaultNamesNoDiag":["Port 1 (WAN)"]},"RecoveryMedia":{"LocateRDMethod":"byLabel","ValidateRDMethod":"usb","FsType":"ext3","SSD":true},"Lcd":"none","DevCodeName":"cputest1","Prototype":true},
{"Familyname":"cputest","DmiMbMfg":"cputest","DmiProdName":"cputest","DmiProdModelRegex":".*","CPU":"someOtherVersion","SerNumField":"system-serial-number","NumDataDisks":1,"Disksize":21474836480,"DiskIsSSD":true,"SwRaidlevel":-1,"Virttype":2,"NICInfo":{"SharedDiagPorts":[],"WANIndex":0,"DefaultNamesNoDiag":["Port 1 (WAN)"]},"RecoveryMedia":{"LocateRDMethod":"byLabel","ValidateRDMethod":"usb","FsType":"ext3","SSD":true},"Lcd":"none","DevCodeName":"cputest2","Prototype":true}]}`

//load embedded data
func getJson() []byte {
	j, err := Asset("appliance.json")
	if err == nil {
		return j
	}
	log.Log("no embedded appliance.json, using default")
	return []byte(aj_default)
}
