// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package disk

import (
	"gprovision/pkg/common/strs"
	"gprovision/pkg/log/testlog"
	"strings"
	"testing"
)

func TestRemoveOpts(t *testing.T) {
	want := "noauto,noexec,noatime"
	had := "noauto,noexec,noatime,nofail"
	bad := []string{"auto", "nofail", "uid="}
	testRemoveOpts(t, want, had, bad)
	had = "auto,noexec,noatime,nofail"
	want = "noexec,noatime"
	testRemoveOpts(t, want, had, bad)
	had = "noexec,noatime,nofail,uid=sdafslfkj,auto"
	testRemoveOpts(t, want, had, bad)
}
func testRemoveOpts(t *testing.T, want, had string, bad []string) {
	got := removeOpts(had, bad...)
	if got != want {
		t.Errorf("want %s, got %s", want, got)
	}
}

func TestFixupRecoveryFS(t *testing.T) {
	/* we test with 9p, as for other fs types it calls existingFsType()
	 * existingFsType() uses blkid and so becomes difficult to test */
	//func (fs Filesystem) FixupRecoveryFS() (err error) {
	fs := Filesystem{}
	fs.isRecovery = false
	fs.mountType = "9p"
	fs.blkdev = strs.RecVolName()
	badopts := "auto,relatime,exec,uid=1,gid=2,user_id=$u,group_id=$g,discard"
	fs.mountOpts = badopts
	fs2 := fs
	e := fs.FixupRecoveryFS()
	if e == nil {
		t.Errorf("FixupRecoveryFS reports success with non-recovery fs")
	}
	if fs.mountOpts != badopts || !strings.Contains(fs.mountOpts, "id") {
		t.Errorf("FixupRecoveryFS modifed opts: \n%s ->\n%s\n", badopts, fs.mountOpts)
	}
	fs2.isRecovery = true
	e = fs2.FixupRecoveryFS()
	if e != nil {
		t.Errorf("FixupRecoveryFS returned %s", e)
	}
	if fs2.mountOpts == badopts || strings.Contains(fs2.mountOpts, "id") {
		t.Errorf("FixupRecoveryFS failed to remove bad opts: %s -> %s", badopts, fs2.mountOpts)
	}
}

//func kBuildNum(out, kpath string) (ver uint64, success bool)
func TestKBuildNum(t *testing.T) {
	data := []struct {
		path    string
		out     string
		ver     uint64
		success bool
	}{
		{"norm_boot", "norm_boot: Linux kernel x86 boot executable bzImage, version 4.11.4-norm_boot (user@host) #57 SMP Fri Sep 8 14:47:37 UTC 2117, RO-rootFS, swap_dev 0x11, Normal VGA", 57, true},
		{"work/norm_boot", "work/norm_boot: Linux kernel x86 boot executable bzImage, version 4.5.6-norm_boot (mark@mark-gentoo) #1 SMP Mon Nov 20 12:26:29 EST 2217, RO-rootFS, swap_dev 0x11, Normal VGA", 1, true},
		{"/boot/kernel-genkernel-x86_64-4.5.6-gentoo-r1", "/boot/kernel-genkernel-x86_64-4.5.6-gentoo-r1: Linux kernel x86 boot executable bzImage, version 4.5.6-gentoo-r1 (root@mark-gentoo) #2 SMP Thu Sep 14 16:30:26 EDT 2017, RO-rootFS, swap_dev 0xE, Normal VGA", 2, true},
		{"work/fr_initramfs/recovery", "work/fr_initramfs/recovery: ELF 64-bit LSB executable, x86-64, version 1 (SYSV), dynamically linked, interpreter /lib64/ld-linux-x86-64.so.2, for GNU/Linux 2.6.32, stripped", 0, false},
		{"", "Linux kernel x86 boot executable bzImage, version 4 #99 blah", 99, true},
		{"", "Linux kernel x86 boot executable bzImage, version 4 #9a blah", 0, false},
		{"", "Linux kernel x86 boot executable bzImage, version 4 99 blah", 0, false},
		{"", "Linux kernel x86 boot executable bzImage, version #4 #99 blah", 0, false},
	}
	for _, line := range data {
		ver, success := kBuildNum(line.out, line.path)
		if ver != line.ver {
			t.Errorf("%s: want ver %d, got %d", line.out, line.ver, ver)
		}
		if success != line.success {
			t.Errorf("%s: want %t, got %t", line.out, line.success, success)
		}
	}
}

//func scoreDisks(in []*Disk, tgtSize, count int, tol float64) (out []*Disk)
func TestScoreDisks(t *testing.T) {
	var list dlist
	var i int64
	for i = 0; i < 10; i++ {
		d := &Disk{
			size:       19500 + i*100,
			identifier: "sd" + string('a'+rune(i)),
		}
		list = append(list, d)
	}
	list = append(list, &Disk{size: 20010, identifier: "sdm"})
	list = append(list, &Disk{size: 512110190592, identifier: "sdn"})
	list = append(list, &Disk{size: 500000000000, identifier: "sdo"})
	list = append(list, &Disk{size: 1000204886016, identifier: "sdp"})
	list = append(list, &Disk{size: 1000204886016, identifier: "sdq"})
	tstIdx := 0
	//                       tgt, req #, resp #, group tol%, abs tol%
	tsdHelper(t, &tstIdx, list, 19990, 2, 2, 1, 1)
	tsdHelper(t, &tstIdx, list, 199900, 2, 0, 1, 1)
	tsdHelper(t, &tstIdx, list, 199900, 2, 2, 1, 100)
	tsdHelper(t, &tstIdx, list, 512110190592, 1, 1, 0, 0)
	tsdHelper(t, &tstIdx, list, 500000000000, 2, 0, 1, 1)
	tsdHelper(t, &tstIdx, list, 500000000000, 2, 0, 1, 1)
	tsdHelper(t, &tstIdx, list, 500000000000, 1, 1, 0, 0)
	tsdHelper(t, &tstIdx, list, 512110190592, 2, 2, 3, 3)
	tsdHelper(t, &tstIdx, list, 512110190592, 2, 0, 2, 3)
	tsdHelper(t, &tstIdx, list, 512110190592, 2, 0, 3, 2)
	tsdHelper(t, &tstIdx, list, 512110190592, 13, 0, 94, 100)
	tsdHelper(t, &tstIdx, list, 512110190592, 13, 0, 100, 99)
	tsdHelper(t, &tstIdx, list, 512110190592, 13, 13, 100, 100)
	tsdHelper(t, &tstIdx, list, 1000204886016, 2, 2, 0, 0)
	tsdHelper(t, &tstIdx, list, 700204886016, 4, 0, 50, 43)
	tsdHelper(t, &tstIdx, list, 700204886016, 4, 4, 51, 43)
	tsdHelper(t, &tstIdx, list, 700204886016, 4, 0, 51, 42)
}
func tsdHelper(t *testing.T, tstIdx *int, list dlist, tgt uint64, reqcount, wcount int, grpTol, absTol uint64) {
	*tstIdx++
	failed := false
	tlog := testlog.NewTestLog(t, true, false)
	res := list.filter(tgt, reqcount, grpTol, absTol)
	if len(res) != wcount {
		t.Errorf("%d: want %d disks, got %d: %s", *tstIdx, wcount, len(res), res)
		failed = true
	}
	for i, d := range res {
		for j, e := range res {
			if i >= j {
				continue
			}
			if d.identifier == e.identifier {
				t.Errorf("%d: two disks with same identifier - %d:%s %d:%s", *tstIdx, i, d.identifier, j, e.identifier)
				failed = true
			}
		}
	}
	if failed {
		t.Logf("%d: list=%s", *tstIdx, list)
		for i, d := range res {
			t.Logf("%d: disk %d: %s %d", *tstIdx, i, d.identifier, d.size)
		}
		tlog.Freeze()
		l := tlog.Buf.String()
		if l != "" {
			t.Log(l)
		}
	}
}
