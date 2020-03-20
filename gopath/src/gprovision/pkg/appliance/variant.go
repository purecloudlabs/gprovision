// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

// Package appliance contains data on various models/revisions of appliance.
//
// This includes sufficient data for identification of a particular variant, as
// well as data on its components where differences may matter.
//
// For example, RAM doesn't matter (except when there is very little of it).
// RAID type matters, but the CPU type and number of cores doesn't.
//
// Build tags
//
// light: light builds are able to query platform_facts.json only; non-light is
// also able to use dmidecode (which requires root).
//
// release: non-release builds include extra functionality for use in testing
// other packages.
//
// Generated code
//
// `go generate` runs go-bindata, encoding files under data/. This is embedded
// in the binary.
package appliance

import (
	"fmt"
	"gprovision/pkg/log"
	"strconv"
	"strings"
)

//type for function to validate recovery device. return nil for match, or error
type validRDfn func(string) error
type ValidateRDEnum int //marshalling function in json.go

const (
	NoValidation ValidateRDEnum = iota
	ValidateUSB
	ValidateSATA
	Validate9P
)

//type for function to locate recovery device; utilizes validRDfn
type locateRDfn func(rm recoveryMediaS) (fsIdent, fsType, fsOpts string)
type LocateRDEnum int //marshalling function in json.go

const (
	LocateByLabel LocateRDEnum = iota
	Locate9PVirt
)

//not sure it really makes sense to have this separate, but for now we will, rather than rewriting again...
type recoveryMediaS struct {
	LocateRDMethod   LocateRDEnum   `json:",omitempty"`
	ValidateRDMethod ValidateRDEnum `json:",omitempty"`
	FsType           string         //single type or comma-delimited list (no spaces), suitable for passing to 'mount -t'. ordered by preference.
	FsOptsAdditional string         `json:",omitempty"` //options for 'mount -o'
	FsOptsOverride   bool           `json:",omitempty"` //if true, FsOptsAdditional replaces defaults rather than adding to it
	SSD              bool           `json:",omitempty"` //if true, adds discard option
}

//contents of this struct assume the interfaces have been sorted by MAC and indexes start at 0
type NICInfo struct {
	// SharedDiagPorts contains a list of _shared_ diag port indexes. This will
	// be empty on platforms where diag ports are isolated from the OS. These
	// ports will be hidden by nic_config, so this should generally be
	// ignored.
	SharedDiagPorts []int

	// WANIndex is the index of the wan port, starting at 0, once diag ports are excluded.
	WANIndex int

	// DefaultNamesNoDiag: port names/aliases. Note that this list only makes sense once
	// diag ports are hidden by nic_config.
	DefaultNamesNoDiag []string

	// MACPrefix overrides the default prefix from strs.MacOUI() with one or more different prefixes.
	// Note that if two prefixes may/will be present, one of which is the default, both must be listed.
	// Format as colon-separated string since json does not support hex.
	MACPrefix []string `json:",omitempty"`
}

// A hidden field of Variant. Hidden/protected because I've been bitten too many times by accidental overwrites.
type Variant_ struct {
	Familyname            string         //like code name, but more generic. lower case. ex: oxcart
	DmiMbMfg, DmiProdName string         //mfg & product name from DMI
	DmiProdModelRegex     string         //regex to match the DMI Product Model field (the SKU shown by `dmidecode -t 1`)
	DmiPMOverrideTyp      int            `json:",omitempty"` //override source of product model data; this is a handle type passed with -t to dmidecode
	DmiPMOverrideFld      string         `json:",omitempty"` //override source of product model data; this is the field name in the human-readable output
	CPU                   string         `json:",omitempty"` //fill out if two models are otherwise indistinguishable, otherwise leave blank.
	SerNumField           string         //'dmidecode -s' field name
	NICInfo               NICInfo        //nic configuration information
	IPMI                  bool           `json:",omitempty"`
	IPMIChannel           int            `json:",omitempty"` //the networked ipmi channel to be queried for IP information (needed by services in the OS)
	NumDataDisks          int            //generally 2 if raid, otherwise 1
	Disksize              uint64         //size in bytes, BLKGETSIZE64
	DiskIsSSD             bool           `json:",omitempty"`
	SwRaidlevel           int            // -1 for no raid, or for HW raid we don't configure
	FakeraidType          string         `json:",omitempty"` //DDF, iMSM (field describes format used by windows; used in data erase process)
	BiosConfigTool        string         `json:",omitempty"`
	IpmiConfigTool        string         `json:",omitempty"`
	Virttype              Virtualization `json:",omitempty"`
	RecoveryMedia         recoveryMediaS //in separate struct
	Lcd                   LcdType        //renamed; now stored as string in json rather than enum value
	DevCodeName           string         //specific code name - ex: Oxcart-12
	Partoffset            string         `json:",omitempty"` //unused?
	Lowmemory             bool           `json:",omitempty"` //true for low-memory devices such as the micro
	Prototype             bool           `json:",omitempty"` //true for prototypes - relax some restrictions
	DiskSTol              uint64         `json:",omitempty"` //allowable tolerance between disks in a group
	DiskTTol              uint64         `json:",omitempty"` //allowable tolerance between any one disk in group and target size
}

//Variant describes a particular model of appliance.
type Variant struct {
	/*
		embedding 'Variant_' with an unexported name hides internal variables,
		which can't otherwise be hidden or json marshal/unmarshal won't see them
		https://golang.org/doc/effective_go.html#embedding
	*/
	i                      Variant_
	mfg, prod, sku, serial string
}

//virtualization type
type Virtualization int

const (
	BareMetal Virtualization = iota
	HyperV
	KVM
	Qemu
	VirtualBox
	VMWare
)

//what type of lcd
type LcdType int

const (
	Cfa635 LcdType = iota
	Cfa631
	NoLCD
	//Caf2k
)

//general consts
const (
	/* standard mount options used on recovery media
	 *
	 * nofail option - system boots even if dev isn't present,
	 * mounts it when it appears. otherwise, there is a long
	 * delay before it goes to a failsafe, single-user mode
	 * (presumably without our services running)
	 *
	 * special codes: $u and $g are replaced by admin's uid and gid, respectively.
	 */
	StandardMountOpts = "auto,noexec,relatime,nofail,uid=$u,gid=$g"
)

var variants []Variant_
var identifiedVariant *Variant

func (v *Variant) HasRaid() bool {
	return v.i.SwRaidlevel != -1
}
func (v *Variant) RaidLevel() int {
	return v.i.SwRaidlevel
}

func (v *Variant) FakeRaidType() string {
	return v.i.FakeraidType
}

func (v *Variant) DataDisks() int {
	return v.i.NumDataDisks
}
func (v *Variant) DiskSize() uint64 {
	return v.i.Disksize
}

func (v *Variant) BiosConfigTool() string {
	return v.i.BiosConfigTool
}
func (v *Variant) IpmiConfigTool() string {
	t := v.i.IpmiConfigTool
	if v.i.IPMI && t == "" {
		t = "internal_ipmi"
	}
	return t
}

func (v *Variant) SerNum() string {
	return v.serial
}

func (v *Variant) Virtual() bool {
	return v.i.Virttype != BareMetal
}

func (v *Variant) VirtType() Virtualization {
	return v.i.Virttype
}

var locateFns = map[LocateRDEnum]locateRDfn{
	LocateByLabel: locateByLabel,
	Locate9PVirt:  locate9PvirtRecov,
}

func (v *Variant) FindRecoveryDev() (fsIdent, fsType, fsOpts string) {
	locate := locateFns[v.i.RecoveryMedia.LocateRDMethod]
	return locate(v.i.RecoveryMedia)
}
func (v *Variant) CheckRecoveryDev(dev string) error {
	return v.i.RecoveryMedia.ValidateFn(dev)
}
func (v *Variant) RecoveryDevVirt() bool {
	return v.i.RecoveryMedia.LocateRDMethod == Locate9PVirt
}

func (v *Variant) Lcd() LcdType {
	return v.i.Lcd
}

func (v *Variant) DeviceCodeName() string {
	return v.i.DevCodeName
}

func (v *Variant) SSD() bool {
	return v.i.DiskIsSSD
}

func (v *Variant) PartOffset() string {
	return v.i.Partoffset
}

func (v *Variant) FamilyName() string {
	return v.i.Familyname
}

func (v *Variant) LowMemory() bool {
	return v.i.Lowmemory
}

var validateFns = map[ValidateRDEnum]validRDfn{
	ValidateUSB:  validateUSB,
	ValidateSATA: validateSATA,
	Validate9P:   validate9P,
}

func (m *recoveryMediaS) ValidateFn(dev string) error {
	if m.ValidateRDMethod == NoValidation {
		return nil
	}
	return validateFns[m.ValidateRDMethod](dev)
}

func (v *Variant) DiagPorts() []int {
	if v == nil {
		return nil
	}
	return v.i.NICInfo.SharedDiagPorts
}

func (v *Variant) HasIPMI() bool {
	return v.i.IPMI
}

func (v *Variant) DiskSetTol() uint64 {
	if v.i.DiskSTol == 0 {
		//default 1%, the limit for mdadm
		return 1
	}
	return v.i.DiskSTol
}
func (v *Variant) DiskTgtTol() uint64 {
	if v.i.DiskTTol == 0 {
		//0 is default for variables omitted from json, but we want to default to 5%
		//side effect: minimum value is 1%, not 0%
		return 5
	}
	return v.i.DiskTTol
}

func (v *Variant) Mfg() string {
	return v.mfg
}
func (v *Variant) Prod() string {
	return v.prod
}
func (v *Variant) SKU() string {
	return v.sku
}

func (v *Variant) PrettyName() string {
	return fmt.Sprintf("%s %s %s", v.mfg, v.prod, v.sku)
}

func (v *Variant) IsPrototype() bool {
	return v.i.Prototype
}
func (v *Variant) MACPrefixes() (prefixes [][]byte) {
	if len(v.i.NICInfo.MACPrefix) == 1 && v.i.NICInfo.MACPrefix[0] == "*" {
		// Other genesys software interprets an empty MACPrefix list to mean
		// that MACs with our OUI are in use and does not use the list contents
		// in any other way. In mfg and factory restore, we filter out any NICs
		// that don't match the list (if it's not empty). Since we can't predict
		// all the prefixes that may be in use by our suppliers we must allow
		// any MAC (i.e. behave as if the list is empty). Satisfy both by using
		// an asterisk in the json, and translating that into an empty list for
		// our purposes.
		return
	}
outer:
	for _, pfx := range v.i.NICInfo.MACPrefix {
		octets := strings.Split(strings.Trim(pfx, ":"), ":")
		var p []byte
		for _, o := range octets {
			v, err := strconv.ParseUint(o, 16, 8)
			if err != nil {
				log.Logf("parsing %s in MAC prefix %s: %s", o, pfx, err)
				continue outer
			}
			p = append(p, byte(v))
		}
		prefixes = append(prefixes, p)
	}
	return
}

func (v *Variant) DefaultPortNames() []string {
	return v.i.NICInfo.DefaultNamesNoDiag
}
func (v *Variant) WANIndex() int {
	return v.i.NICInfo.WANIndex
}
