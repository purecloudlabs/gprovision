// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package uefi

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"unicode/utf16"

	"github.com/purecloudlabs/gprovision/pkg/common/strs"
	"github.com/purecloudlabs/gprovision/pkg/log"
)

type BootLabel string

const (
	BootLabelFR    BootLabel = "Forced Factory Restore"
	BootLabelNorm            = "Normal Boot"
	BootLabelErase           = "Data Erase (DANGER!)"
)

type BootEntry struct {
	Device   string
	PartNum  uint
	Label    BootLabel
	AbsPath  string
	Args     string
	autoBoot bool
}

/* use forward slashes in -l path.
   binary need not have efi extension.
   args (-@) only work in a file, and the file MUST be UTF-16.

   echo -n -e "a\0r\0g\0=\0v\0a\0l\0" >args && \
   efibootmgr -c -d /dev/sda -p 1 -L "norm_boot" -l "/norm_boot" -@ args
*/

//add a boot entry. efibootmgr sets it as primary
func AddBootEntry(b BootEntry) {
	create := exec.Command("efibootmgr")
	if b.autoBoot {
		//add variable, add to boot order
		create.Args = append(create.Args, "-c")
	} else {
		//add variable, don't add to boot order
		create.Args = append(create.Args, "-C")
	}
	create.Args = append(create.Args, "-d", b.Device, "-p", fmt.Sprintf("%d", b.PartNum))
	create.Args = append(create.Args, "-L", string(b.Label), "-l", b.AbsPath)
	if len(b.Args) > 0 {
		args := utf16.Encode([]rune(b.Args))
		f, err := ioutil.TempFile("", "bootargs")
		if err != nil {
			log.Logf("cannot create boot arg file: %s", err)
			log.Fatalf("failed to write args for boot entry")
		}
		defer os.Remove(f.Name())
		defer f.Close()
		for _, c := range args {
			var u16 [2]byte
			//note - swapping byte order
			u16[1] = byte(c >> 8)
			u16[0] = byte(c & 0xff)
			if _, err := f.Write(u16[:]); err != nil {
				log.Logf("writing boot entry %s: %s", b, err)
			}
		}
		create.Args = append(create.Args, "-@", f.Name())
	}
	out, err := create.CombinedOutput()
	if err != nil {
		log.Logf("cannot add boot entry %v: %s\nout: %s", b, err, string(out))
		log.Fatalf("failed to add boot entry")
	} else {
		log.Logf("adding boot entry for %s", string(b.Label))
	}
}

func RemoveBootEntry(num uint16) error {
	rm := exec.Command("efibootmgr", "--bootnum", fmt.Sprintf("%04X", num), "--delete-bootnum")
	out, err := rm.CombinedOutput()
	if err != nil {
		log.Logf("executing %v: error %s\nout: %s", rm.Args, err, string(out))
	}
	return err
}

/*
efibootmgr's output is thoroughly horrible
read efi variables directly instead of using efibootmgr to show entries
*/

//for uefi, do we care about having multiple partitions each containing norm_boot?
//secure boot?

func (entries BootEntryVars) OursPresent() bool {
	ours := entries.Ours()
	if len(ours) != 3 {
		return false
	}
	norm, fr, erase := ours.CheckMissing()
	return norm && fr && erase
}

func (entries BootEntryVars) CheckMissing() (haveNormal, haveFR, haveErase bool) {
	for _, e := range entries {
		switch BootLabel(e.Description) {
		case BootLabelFR:
			haveFR = true
		case BootLabelNorm:
			haveNormal = true
		case BootLabelErase:
			haveErase = true
		}
	}
	return
}

func (entries BootEntryVars) FixMissing(baseEntry BootEntry, extraOpts string) {
	n, f, e := entries.CheckMissing()
	fixMissing(baseEntry, n, f, e, extraOpts)
}
func fixMissing(baseEntry BootEntry, haveNormal, haveFR, haveErase bool, extraOpts string) {
	if !haveErase {
		b := baseEntry
		b.Args = strs.EraseEnv() + "=1"
		b.Label = BootLabelErase
		AddBootEntry(b)
	}
	if !haveFR {
		b := baseEntry
		b.Label = BootLabelFR
		AddBootEntry(b)
	}
	if !haveNormal {
		b := baseEntry
		/* for legacy units, we pass the root volume's uuid as a boot arg. however,
		   that uuid will change with each factory restore as the fs is created anew.
		   if the uefi boot entry is updated, that means the "nvram" (actually flash)
		   is written, and getting into a factory restore loop could wear out the flash.
		   to avoid this, we use LABEL=... instead
		*/
		b.Args = strings.Join([]string{b.Args, "real_root=LABEL=" + strs.PriVolName(), extraOpts}, " ")
		b.Label = BootLabelNorm
		b.autoBoot = true
		AddBootEntry(b)
	}
}
