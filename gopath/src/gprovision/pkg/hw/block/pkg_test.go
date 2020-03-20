// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package block

import (
	"gprovision/pkg/common/strs"
	"os"
	"testing"
)

type blkTestData struct {
	line      string
	want      BlkInfo
	expectErr bool
}

//func parseBlkidOut(dev string, out []byte) (bi BlkInfo, err error)
func TestBlkIdParse(t *testing.T) {
	testData := []blkTestData{
		{`/dev/sda1: LABEL="boot0" UUID="dfdc3f4f-5eeb-48b1-9a9a-d0440383408a" TYPE="ext4"`,
			//     FsType,            UUID,               Partition, PartUUID, Label, Usage
			BlkInfo{FsExt4, "dfdc3f4f-5eeb-48b1-9a9a-d0440383408a", true, "", "boot0", "filesystem", ""}, false},
		{`/dev/sda2: LABEL="` + strs.PriVolName() + `" UUID="8532944c-0c9a-4c47-82fa-0eabbceb6c8e" TYPE="ext4"`,
			BlkInfo{FsExt4, "8532944c-0c9a-4c47-82fa-0eabbceb6c8e", true, "", strs.PriVolName(), "filesystem", ""}, false},
		{`/dev/sdb1: LABEL="` + strs.RecVolName() + `" UUID="86df13e4-bece-4d93-949e-7b9564946f68" TYPE="ext3"`,
			BlkInfo{FsExt4, "86df13e4-bece-4d93-949e-7b9564946f68", true, "", strs.RecVolName(), "filesystem", ""}, false},
		{`/dev/sda1: LABEL="gentoo" UUID="ed2d36e3-a3d9-408c-9255-897a010a783b" TYPE="ext4" PARTUUID="0006bbe0-01"`,
			BlkInfo{FsExt4, "ed2d36e3-a3d9-408c-9255-897a010a783b", true, "0006bbe0-01", "gentoo", "filesystem", ""}, false},
		{`/dev/sdb1: LABEL="boot" UUID="c4deb5ac-0045-4492-bdab-f50ec082ccd4" TYPE="ext2" PARTUUID="b3c23f2d-01"`,
			BlkInfo{FsExt4, "c4deb5ac-0045-4492-bdab-f50ec082ccd4", true, "b3c23f2d-01", "boot", "filesystem", ""}, false},
		{`/dev/sdb3: PARTUUID="b3c23f2d-03"`,
			BlkInfo{FsUnknown, "", true, "b3c23f2d-03", "", "", ""}, false},
		{`/dev/sdb5: LABEL="swap" UUID="61d87d6b-a51d-4baa-ab7f-c44e2ffcbf05" TYPE="swap" PARTUUID="b3c23f2d-05"`,
			BlkInfo{FsUnknown, "61d87d6b-a51d-4baa-ab7f-c44e2ffcbf05", true, "b3c23f2d-05", "swap", "", ""}, false},
		{`/dev/sdb6: LABEL="data" UUID="eb52a252-cd84-43ea-9d9d-25533d10b6f2" TYPE="ext4" PARTUUID="b3c23f2d-06"`,
			BlkInfo{FsExt4, "eb52a252-cd84-43ea-9d9d-25533d10b6f2", true, "b3c23f2d-06", "data", "filesystem", ""}, false},
		{`/dev/sdb1: LABEL="System Reserved" UUID="A4C465CBC4659FF2" TYPE="ntfs" PARTUUID="3c54d5ca-01"`,
			BlkInfo{FsNtfs, "A4C465CBC4659FF2", true, "3c54d5ca-01", "System Reserved", "filesystem", ""}, false},
		{`/dev/sdb2: UUID="4C4267A142678F10" TYPE="ntfs" PARTUUID="3c54d5ca-02"`,
			BlkInfo{FsNtfs, "4C4267A142678F10", true, "3c54d5ca-02", "", "filesystem", ""}, false},
		{`/dev/sdb2: UUID="4C4267A142678F10" TYPE="ext1" PARTUUID="3c54d5ca-02"`,
			BlkInfo{FsUnknown, "4C4267A142678F10", true, "3c54d5ca-02", "", "", ""}, false},
		{`/dev/sdb2: UUID="ntfs4C4267A142678F10" TYPE="ext4" PARTUUID="ntfs3c54d5ca-02"`,
			BlkInfo{FsExt4, "ntfs4C4267A142678F10", true, "ntfs3c54d5ca-02", "", "filesystem", ""}, false},
		{``, BlkInfo{FsUnknown, "", false, "", "", "", ""}, true},
		{`tstdev: USAGE="none" UUID='magic' PARTUUID="majik" TYPE='exfat'`,
			BlkInfo{FsExfat, "magic", true, "majik", "", "none", ""}, false},
		{`/dev/nvme0n1p1: SEC_TYPE="msdos" LABEL_FATBOOT="ESP" LABEL="ESP" UUID="3AB2-ADD3" TYPE="vfat" PARTLABEL="ESP" PARTUUID="81635ccd-1b4f-4d3f-b7b7-f78a5b029f35"`,
			BlkInfo{FsFat, "3AB2-ADD3", true, "81635ccd-1b4f-4d3f-b7b7-f78a5b029f35", "ESP", "filesystem", ""}, false},
	}
	for i, o := range testData {
		binfo, err := parseBlkidOut([]byte(o.line))
		if (err != nil) != o.expectErr {
			t.Errorf("%d %s\nexpectErr=%t, err=%s", i, o.line, o.expectErr, err)
		}
		if binfo != o.want {
			t.Errorf("%d %s\ngot  %#v\nwant %#v", i, o.line, binfo, o.want)
		}
	}
}

type partPair struct{ root, part string }

func findBlk() (partPair, error) {
	var possibilities = []partPair{
		{"nvme0n1", "p1"},
		{"xvda", "1"}, //aws-only; can be a symlink to nvme (breaks the test) so try after nvme
		{"sda", "1"},
	}
	var pair partPair
	for _, p := range possibilities {
		_, err := os.Stat("/dev/" + p.root)
		if err == nil {
			return p, nil
		}
	}
	return pair, os.ErrInvalid
}

//func PartParent(dev string) string
func TestParent(t *testing.T) {
	pair, err := findBlk()
	if err != nil {
		t.Error(err)
	}
	parent := PartParent(pair.root + pair.part)
	if parent != pair.root {
		t.Errorf("expected %s, got %s", pair.root, parent)
	}
}

//func IsDev(dev string) bool
func TestIsDev(t *testing.T) {
	pair, err := findBlk()
	if err != nil {
		t.Error(err)
	}
	if IsDev("") {
		t.Error("want false")
	}
	if IsDev(pair.root + pair.part) {
		t.Error("want false")
	}
	if !IsDev(pair.root) {
		t.Error("want true")
	}
}

//func IsPart(dev string) bool
func TestIsPart(t *testing.T) {
	pair, err := findBlk()
	if err != nil {
		t.Error(err)
	}
	if !IsPart(pair.root + pair.part) {
		t.Error("want true")
	}
	if IsPart(pair.root) {
		t.Error("want false")
	}
	if IsPart("") {
		t.Error("want false")
	}
}
