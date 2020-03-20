// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package qa

import (
	"fmt"
	"gprovision/pkg/hw/block"
	"gprovision/pkg/log"
	"gprovision/pkg/mfg/mfgflags"
)

type Disk struct {
	Size             uint64 //value reported by blockdev --getsize64. MUST be > 0, as 0 is treated specially
	SizeTolerancePct uint64 `json:",omitempty"` //5 = 5%
	Vendor           string `json:",omitempty"` //hdparm, cat /sys/block/sda/device/vendor, vendor id
	Model            string `json:",omitempty"` //hdparm, cat /sys/block/sda/device/model, model id
	Quantity         int    `json:",omitempty"`
	dev              string //list of devices matching
}

func (d Disk) String() string {
	return fmt.Sprintf("Disk %s/%s, size %d +- %d%%, qty %d, devs '%s'", d.Vendor, d.Model, d.Size, d.SizeTolerancePct, d.Quantity, d.dev)
}

type Disks []*Disk

//return total number of disks in list
func (ds Disks) Qty() (q int) {
	for _, d := range ds {
		q += d.Quantity
	}
	return
}

type RecoveryDisk Disk

func (r *RecoveryDisk) Populate() {
	(*Disk)(r).Populate()
}
func (dsk *Disk) Populate() {
	devs := block.Devices()
	dsk.populate(devs)
}
func (dsk *Disk) populate(devs []block.BlockDev) {
	dsk.Quantity = 0
	for _, dev := range devs {
		if dev.Vendor == dsk.Vendor && dev.Model == dsk.Model {
			if mfgflags.Verbose {
				log.Logf("match for %s - %s", dev.Model, dev.Name)
			}
			dsk.Quantity += 1
			if dsk.Quantity > 1 {
				dsk.dev += ", " + dev.Name
				if dev.Size != dsk.Size {
					log.Logf("multiple devices, different sizes - %s", dev)
					dsk.Size = 0
				}
				continue
			}
			dsk.dev = dev.Name
			dsk.Size = dev.Size
		} else {
			if mfgflags.Verbose {
				log.Logf("%s != %s", dev, dsk.Model)
			}
		}
	}
	if dsk.Quantity == 0 {
		log.Logf("no devices with vendor/model %s/%s", dsk.Vendor, dsk.Model)
	}
}

func (required RecoveryDisk) Compare(detected RecoveryDisk) (errors int) {
	if required.Quantity == 0 {
		required.Quantity = 1
	}
	errors = Disk(required).Compare(Disk(detected))
	if errors == 0 {
		log.Msg("+++ Recovery Disk: match +++")
	} else {
		log.Msgf("!!! Recovery Disk: %d errors !!!", errors)
	}
	return
}

func (required Disk) Compare(detected Disk) (errors int) {
	if required.Size == 0 {
		log.Logf("disk %s/%s required size is 0, which is not allowed", required.Vendor, required.Model)
		return 1
	}
	if detected.Quantity != required.Quantity {
		log.Msgf("'%s': want quantity %d, got %d", required.Model, required.Quantity, detected.Quantity)
		return 1
	}
	if detected.Size == 0 && detected.Quantity > 1 {
		log.Logf("multiple devices have the desired vendor/model, but differing sizes")
		return 1
	}
	if !block.SizeToleranceMatch(detected.Size, required.Size, required.SizeTolerancePct) {
		log.Logf("size out of tolerance for vendor/model %s/%s - want %d, got %d", detected.Vendor, detected.Model, required.Size, detected.Size)
		return 1
	}
	return 0
}

type MainDisk Disk

func (m MainDisk) String() string { return Disk(m).String() }

type MainDisks []*MainDisk

func (mds MainDisks) String() (s string) {
	for _, md := range mds {
		s += md.String() + "\n"
	}
	return
}
func (mds MainDisks) Qty() (q int) {
	for _, d := range mds {
		q += d.Quantity
	}
	return
}

type MainDiskConfigs []MainDisks

func PopulateDisks(cfgs MainDiskConfigs) (mdisks MainDisks, cfgIdx int) {
	found := FoundDisks()
	return populateDisks(cfgs, found)
}
func populateDisks(cfgs MainDiskConfigs, found Disks) (mdisks MainDisks, cfgIdx int) {
	cfgIdx = -1
	for i, cfg := range cfgs {
		if cfgMatch(cfg, found) {
			//loop over list in cfg, but take size from found
			for _, c := range cfg {
				md := new(MainDisk)
				*md = *c
				md.Size = 0
				for _, d := range found {
					if (*Disk)(md).compareMV(d) {
						md.Size = d.Size
						md.Quantity = d.Quantity
					}
				}
				if md.Size == 0 {
					log.Fatalf("populateDisks: something went wrong. cfg=%v found=%v", cfg, found)
				}
				mdisks = append(mdisks, md)
			}
			cfgIdx = i
			break
		} else if mfgflags.Verbose {
			log.Logf("disk configs: no match found")
		}
	}
	if mfgflags.Verbose {
		log.Logf("MainDisks:\n%s", mdisks)
	}
	return
}

func cfgMatch(cfg MainDisks, foundDisks Disks) bool {
	//because foundDisks will include recovery, len(found) == len(cfg) + 1 always
	if cfg.Qty()+1 != foundDisks.Qty() {
		return false
	}
	var remaining = make(MainDisks, len(cfg)) //disks from cfg that haven't been found yet
	copy(remaining, cfg)
	notInCfg := 0
outer:
	for _, d := range foundDisks {
		for i, r := range remaining {
			if r.Model == d.Model &&
				r.Vendor == d.Vendor {
				if r.Quantity > d.Quantity {
					r.Quantity -= d.Quantity
					continue outer
				} else if r.Quantity == d.Quantity {
					//matches
					remaining = append(remaining[:i], remaining[i+1:]...)
					continue outer
				}
			}
		}
		notInCfg++
	}
	if notInCfg != 1 {
		//should always be 1 because of recovery drive
		return false
	}
	return len(remaining) == 0
}

//compare model, vendor
func (a Disk) compareMV(b *Disk) bool {
	return a.Model == b.Model && a.Vendor == b.Vendor
}

func (required MainDisk) Compare(detected MainDisk) (errors int) {
	return Disk(required).Compare(Disk(detected))
}

func (required MainDiskConfigs) Compare(detected MainDisks, idx int) (errors int) {
	if idx == -1 {
		errors = 1
		log.Msgf("No configuration from mfgData matches detected devices")
		log.Msgf("!!! Main Disks: %d errors !!!", errors)
		return
	}
	errors = required[idx].Compare(detected)
	if errors == 0 {
		log.Logf("Detected disks match MainDiskConfigs[%d]: %s", idx, required[idx])
	}
	return
}

func (required MainDisks) Compare(detected MainDisks) (errors int) {
	for i, r := range required {
		if i >= len(detected) {
			errors++
			log.Logf("more disks required than detected: got %d, want %d\ndetected: %v", len(detected), len(required), detected)
			break
		}
		errors += r.Compare(*detected[i])
		if detected[i].Quantity != r.Quantity {
			log.Logf("Main disk %s/%s: wrong number detected - want %d, got %d. device(s) matching: %s",
				r.Vendor, r.Model, r.Quantity, detected[i].Quantity, detected[i].dev)
			errors++
		}
	}
	if len(detected) > len(required) {
		errors++
		log.Logf("more disks detected than required: got %d, want %d", len(detected), len(required))
	}
	if errors == 0 {
		log.Msg("+++ Main Disks: match +++")
	} else {
		log.Msgf("!!! Main Disks: %d errors !!!", errors)
	}
	return
}
func FoundDisks() (disks []*Disk) {
	devs := block.Devices()
	for _, dev := range devs {
		match := false
		for _, d := range disks {
			if d.Model == dev.Model && d.Vendor == dev.Vendor && d.Size == dev.Size {
				match = true
				d.Quantity += 1
				break
			}
		}
		if !match {
			disks = append(disks, &Disk{
				Size:             dev.Size,
				SizeTolerancePct: 0,
				Vendor:           dev.Vendor,
				Model:            dev.Model,
				Quantity:         1,
			})
		}
	}
	return
}
func DumpDisks() {
	out := "--- block device dump: ---\n"
	devs := block.Devices()
	for _, dev := range devs {
		out += dev.String() + "\n"
	}
	log.Logf(out + "--- end block device dump ---\n")

}
