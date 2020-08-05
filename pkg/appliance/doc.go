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
// also able to use dmidecode (which accesses /dev/mem and thus requires root).
//
// release: non-release builds include extra functionality for use in testing
// other packages.
//
// Generated code
//
// `go generate` runs go-bindata, encoding files and embedding them in the binary.
//
//    //go:generate ../../bin/go-bindata -tags !light -prefix=../../proprietary/data/appliance -pkg=$GOPACKAGE ../../proprietary/data/appliance
//
// In this repo, there are no files in the referenced dir, so a lookup of
// embedded file `appliance.json` returns nothing. In this case we fall back to
// the contents of string `aj_default`, defined in identify.go.
//
// To override aj_default, create a file appliance.json in the dir listed above
// ($GOPATH/src/github.com/purecloudlabs/gprovision/proprietary/data/appliance) with content like the
// following, a portion of aj_default. Note that appliance.json overrides
// aj_default, rather than adding to it.
//
//	{
//	  "Variants": [
//	    {
//	      "DevCodeName": "QEMU-mfg-test",
//	      "Familyname": "qemu",
//	      "DmiMbMfg": "GPROV_QEMU",
//	      "DmiProdName": "mfg_test",
//	      "DmiProdModelRegex": ".*",
//	      "SerNumField": "system-serial-number",
//	      "NumDataDisks": 1,
//	      "Disksize": 21474836480,
//	      "DiskIsSSD": true,
//	      "SwRaidlevel": -1,
//	      "Virttype": 2,
//	      "NICInfo": {
//	        "SharedDiagPorts": [],
//	        "WANIndex": 0,
//	        "DefaultNamesNoDiag": [
//	          "Port 1 (WAN)"
//	        ]
//	      },
//	      "RecoveryMedia": {
//	        "LocateRDMethod": "byLabel",
//	        "ValidateRDMethod": "usb",
//	        "FsType": "ext3",
//	        "SSD": true
//	      },
//	      "Lcd": "none",
//	      "Prototype": true
//	    },
//	  ]
//	}
//
// DevCodeName is used in many places, including in manufData to identify which
// set of hardware specs to use and the additional config steps, if any, to
// apply.
//
// Fields such as DmiMbMfg, DmiProdName, and DmiProdModelRegex are used to
// determine a match between the given Variant and the current device.
//
// Other fields tell the application what to expect in terms of existing
// hardware, and in some cases influence what actions are taken - for example,
// SwRaidlevel impacts whether we try to create a raid array and what type of
// array is created.
//
// Appliance json compared to manufData
//
// ManufData, used by the mfg app, has some similarities. Both impose
// restrictions on what hardware must be present, but the intent is that
// appliance json be less restrictive. It must _never_ be more restrictive, else
// a device could be manufactured successfully but fail to factory restore.
//
// During manufacture, the appliance json is first used to identify what the
// device _probably_ is, then manufData.json is used to ensure that the device
// exactly matches everything you deem important.
//
package appliance
