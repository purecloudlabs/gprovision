// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

//Package block contains functions dealing with linux block devices and the underlying hardware.
package block

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/purecloudlabs/gprovision/pkg/common/strs"
	"github.com/purecloudlabs/gprovision/pkg/log"

	"github.com/google/shlex"
)

var Verbose bool

func parseBlkidOut(out []byte) (binfo BlkInfo, err error) {
	split := strings.Split(string(out), ":")
	if len(split) != 2 {
		err = fmt.Errorf("can't parse %s", string(out))
		return
	}
	elements, err := shlex.Split(split[1])
	if err != nil {
		return
	}
	for _, e := range elements {
		kv := strings.Split(e, "=")
		if len(kv) != 2 {
			log.Logf("blkid %s: can't parse %s, skipping", split[0], e)
			continue
		}
		//shlex removes spaces and quotes - we don't need to
		k, v := kv[0], kv[1]

		switch strings.ToUpper(k) {
		case "UUID":
			binfo.UUID = v
		case "TYPE":
			binfo.FsType = FsFromStr(v)
		case "LABEL":
			binfo.Label = v
		case "PARTUUID":
			binfo.Partition = true
			binfo.PartUUID = v
		case "USAGE":
			binfo.Usage = v
		default:
			if Verbose {
				log.Logf("blkid %s: ignoring %s", split[0], e)
			}
		}
	}
	if binfo.FsType.Recognized() {
		binfo.Partition = true
		if binfo.Usage == "" {
			binfo.Usage = "filesystem"
		}
	}
	return
}

type FsType int

const (
	FsUnknown FsType = iota
	FsExt4
	FsNtfs
	FsFat
	FsExfat
)

func FsFromStr(s string) FsType {
	/* some of these probably won't ever be encountered */
	switch strings.ToLower(s) {
	case "ext2":
		fallthrough
	case "ext3":
		fallthrough
	case "ext4":
		return FsExt4
	case "ntfs":
		fallthrough
	case "ntfs-3g":
		return FsNtfs
	case "fat":
		fallthrough
	case "vfat":
		return FsFat
	case "exfat":
		return FsExfat
	}
	return FsUnknown
}
func (f FsType) String() (t string) {
	switch f {
	case FsUnknown:
		t = "unknown"
	case FsExt4:
		t = "ext4"
	case FsNtfs:
		t = "ntfs"
	case FsFat:
		t = "vfat"
	case FsExfat:
		t = "exfat"
	default:
		t = "fsType VALUE OUT OF RANGE"
	}
	return
}

func (f FsType) Recognized() bool {
	return f == FsExt4 || f == FsNtfs || f == FsFat || f == FsExfat
}

type BlkInfo struct {
	FsType    FsType
	UUID      string
	Partition bool
	PartUUID  string
	Label     string
	Usage     string
	Device    string
}

func GetInfo(device string) (bi BlkInfo, err error) {
	blkid := exec.Command("/sbin/blkid", device)
	out, err := blkid.CombinedOutput()
	if err != nil {
		log.Logf("error %s executing %v\noutput:%s\n", err, blkid.Args, out)
		return
	}
	bi, err = parseBlkidOut(out)
	bi.Device = device
	return
}

func DetermineFSType(device string) FsType {
	bi, err := GetInfo(device)
	if err != nil {
		log.Logf("failed to recognize fs on %s", device)
	}
	return bi.FsType
}

//a function that returns false if given bi should be filtered out
type BlkIncludeFn func(bi BlkInfo) bool

func BFiltNotRecovery(bi BlkInfo) bool {
	accept := bi.Label != strs.RecVolName()
	if Verbose {
		log.Logf("BFiltNotRecovery: dev=%s accept=%t", bi.Device, accept)
	}
	return accept
}

//return a BlkInfo for each blockdevice containing a filesystem we recognize
func GetFilesystems(blkfilter BlkIncludeFn, devfilter DevIncludeFn) []BlkInfo {
	var infos []BlkInfo
	for _, d := range FilterBlockDevs(devfilter) {
		bi, err := GetInfo(d)
		if err != nil {
			continue
		}
		if blkfilter != nil && !blkfilter(bi) {
			continue
		}
		infos = append(infos, bi)
	}
	return infos
}
