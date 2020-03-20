// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

// +build !light

package disk

import (
	"bytes"
	"fmt"
	"gprovision/pkg/appliance"
	"gprovision/pkg/common/strs"
	futil "gprovision/pkg/fileutil"
	"gprovision/pkg/hw/uefi"
	"gprovision/pkg/log"
	"io/ioutil"
	"os"
	"os/exec"
	fp "path/filepath"
	"strconv"
	"strings"
	"text/template"
)

//this file contains code related to making the system bootable - legacy or uefi

//determine whether system is uefi or legacy and make it bootable. returns list of additional partitions for fstab
func MakeBootable(disks []*Disk, mainFS, recov *Filesystem, platform *appliance.Variant, extraOpts string) []*Filesystem {
	if uefi.BootedUEFI() {
		return ConfigUEFIBoot(recov, nil, false, extraOpts)
	}
	return WriteLegacyBootParts(disks, mainFS, recov, platform, extraOpts)
}

//add missing boot entries or overwrite all
//return the ESP partition as a Filesystem (for adding to fstab)
//in recovery, boot entries should be correct
func ConfigUEFIBoot(recov, ESP *Filesystem, overwriteBootEnts bool, extraOpts string) (bootParts []*Filesystem) {
	log.Logf("configuring UEFI boot entries...")
	if recov.mountPoint == "" {
		log.Fatalf("recov mountpoint must be set")
	}
	if ESP == nil {
		_, err := os.Stat("/dev/disk/by-label/ESP")
		if err != nil {
			log.Fatalf("uefi but no ESP partition??")
		}
		ESP = &Filesystem{
			blkdev:     "/dev/disk/by-label/ESP",
			mountPoint: fp.Join(recov.Path(), "ESP"),
			mountType:  "vfat",
			mountOpts:  appliance.StandardMountOpts,
			formatted:  true,
		}
	}
	err := os.Mkdir(ESP.mountPoint, 0777)
	if err != nil && !os.IsExist(err) {
		log.Logf("cannot create ESP mountpoint: %s", err)
	}
	ESP.Mount()
	ESP.SetMountpoint(fp.Join(recov.mountPoint, "ESP"))
	bootParts = append(bootParts, ESP)
	entries := uefi.AllBootEntryVars()
	if !overwriteBootEnts && entries.OursPresent() {
		log.Logf("boot entries present, not overwriting")
		return
	}
	if overwriteBootEnts {
		log.Logf("removing our boot entries")
		for _, e := range entries.Ours() {
			log.Logf("remove %s", e.Description)
			err = e.Remove()
			if err != nil {
				log.Logf("can't remove %s: %s", e.Description, err)
			}
		}
		//re-read
		entries = uefi.AllBootEntryVars()
	}
	dev, err := fp.EvalSymlinks(ESP.blkdev)
	if err != nil {
		log.Logf("resolving %s: %s", ESP.blkdev, err)
	}
	part, err := strconv.ParseUint(dev[len(dev)-1:], 10, 8)
	if err != nil {
		log.Logf("determining part# for %s: %s", ESP.blkdev, err)
		log.Fatalf("can't determine part# for ESP")
	}
	tmpl := uefi.BootEntry{
		Device:  dev[:len(dev)-1],
		PartNum: uint(part),
		AbsPath: "/" + strs.BootKernel(),
	}
	tmpl.Args = "quiet"
	if args := kArgsFromEnv(false); len(args) > 0 {
		tmpl.Args = args
	}
	log.Logf("fixing missing boot entries...")
	entries.FixMissing(tmpl, extraOpts)
	log.Logf("done configuring uefi boot")
	return
}

//find recovery volume by looking for the fs label
func FindRecovery(platform *appliance.Variant) (recovery *Filesystem) {
	if platform == nil {
		log.Msg("unrecognized platform")
		return
	}
	fsIdent, fsType, fsOpts := platform.FindRecoveryDev()
	log.Msgf("recovery volume: %s", fsIdent)
	if len(fsIdent) > 0 {
		recovery = new(Filesystem)
		recovery.isRecovery = true
		recovery.blkdev = fsIdent
		recovery.formatted = true
		recovery.mountOpts = fsOpts
		recovery.mountType = fsType
		recovery.mountPoint = "/mnt/recov"
		err := recovery.FixupRecoveryFS()
		if err != nil {
			log.Logf("%s", err)
			log.Fatalf("recovery fixup")
		}
	}
	return
}

func CreateRecovery(platform *appliance.Variant, recoverySize uint64, extraOpts string) (recov *Filesystem) {
	c := platform.RecoveryCandidates(recoverySize)
	//fail unless we've found exactly one candidate
	if len(c) != 1 {
		log.Logf("failed to identify recovery drive. candidates: %#v", c)
		return
	}
	d := &Disk{
		identifier: fp.Base(c[0]),
	}
	log.Msgf("using %s as %s", d.identifier, strs.RecVolName())
	err := PartitionRecovery(d)
	if err != nil {
		log.Fatalf("partitioning recovery: %s", err)
	}
	//reread partition table?
	// RECOVERY must be last partition on device
	dev := "/dev/" + d.identifier + fmt.Sprint(d.numParts)
	recov = &Filesystem{
		blkdev:     dev,
		isRecovery: true,
		mountType:  "ext3",
	}

	_ = recov.Format(strs.RecVolName())

	recov.mountPoint = "/mnt/recov"
	recov.Mount()
	if d.numParts == 2 {
		//create Efi System Partition
		esp := &Filesystem{
			blkdev:    "/dev/" + d.identifier + "1",
			mountType: "vfat",
		}
		_ = esp.Format("ESP")
		esp.mountPoint = recov.mountPoint + "/ESP"
		esp.Mount()
		ConfigUEFIBoot(recov, esp, true, extraOpts)
	} else {
		recov.InstallGrub4Dos()
	}
	return
}

//install grub4dos to MBR
func (fs *Filesystem) InstallGrub4Dos() {
	dev := fs.UnderlyingDevice()
	if dev == "" {
		log.Fatalf("grub4dos install: bad device")
	}
	//bootlace
	lace := exec.Command("/g4d/bootlace64.com", "--no-backup-mbr", "--boot-prevmbr-last", "--time-out=0", "/dev/"+dev)
	out, err := lace.CombinedOutput()
	if err != nil {
		log.Logf("error %s running %#v\noutput:\n%s", err, os.Args, out)
		log.Fatalf("bootlace error")
	}

	err = futil.CopyFile("/g4d/grldr", fs.Path()+"/grldr", 0)
	if err != nil {
		log.Fatalf("error copying grldr")
	}
	menutmpl, err := Asset("menu.lst")
	if err != nil {
		log.Log("using default menu.lst")
		//note, you likely want to set a password on the menu
		menutmpl = []byte(defaultMenu)
	}
	bd := bootData{
		HddFile:  hddBootFile,
		FallFile: fallbackBootFile,
		Kernel:   strs.BootKernel(),
	}
	menu, err := bd.processTemplate(menutmpl, "menu.lst")
	if err != nil {
		panic(fmt.Sprintf("error executing template for menu.lst: %s", err))
	}

	menu = fixupGrub(menu)
	err = ioutil.WriteFile(fs.Path()+"/menu.lst", menu, 0644)
	if err != nil {
		log.Fatalf("creating menu.lst: %s", err)
	}
}

//apply fixes to grub menu files. used in development and integ tests.
func fixupGrub(in []byte) []byte {
	hasConsole := bytes.Contains(in, []byte("console="))
	args := kArgsFromEnv(hasConsole)
	return bytes.Replace(in, []byte("quiet"), []byte(args), -1)
}

//translate env vars into kernel args
func kArgsFromEnv(hasConsole bool) string {
	var args []string
	if ie := os.Getenv(strs.IntegEnv()); len(ie) > 0 {
		if !hasConsole {
			args = append(args, "console=ttyS0", "console=tty0")
		}
		args = append(args, fmt.Sprintf("%s=%s", strs.IntegEnv(), ie))
		if ve := os.Getenv(strs.VerboseEnv()); len(ve) > 0 {
			args = append(args, fmt.Sprintf("%s=%s", strs.VerboseEnv(), ve))
		}
	}
	if ko := os.Getenv(K_OVERRIDE); len(ko) > 0 {
		args = append(args, fmt.Sprintf("%s=%s", K_OVERRIDE, ko))
	}
	return strings.Join(args, " ")
}

var fallbackBootFile = "GPROVFBK.boot"

func (fs *Filesystem) WriteFallbackBootMenu() {
	rfm, err := Asset(fallbackBootFile)
	if err != nil {
		log.Logf("using default content for %s", fallbackBootFile)
		rfm = []byte(defaultFallback)
	}
	bd := bootData{
		FallFile: fallbackBootFile,
		Kernel:   strs.BootKernel(),
	}
	out, err := bd.processTemplate(rfm, fallbackBootFile)
	if err != nil {
		panic(fmt.Sprintf("error executing template for %s: %s", fallbackBootFile, err))
	}
	p := fp.Join(fs.Path(), fallbackBootFile)
	if err = ioutil.WriteFile(p, fixupGrub(out), 0644); err != nil {
		log.Logf("error writing %s: %s", fallbackBootFile, err)
	}
}

var hddBootFile = "GPROVHDD.boot"

type bootData struct {
	Root_uuid, Extra_opts string //hdd boot file only
	Kernel                string //kernel name
	HddFile               string //name of menu on primary disk
	FallFile              string //name of fallback menu on recovery volume
}

func (bd *bootData) processTemplate(in []byte, name string) ([]byte, error) {
	t := template.Must(template.New(name).Parse(string(in)))
	out := new(bytes.Buffer)
	err := t.Execute(out, bd)
	if err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func finalizeGrubConf(root_uuid, extra_opts string) []byte {
	hddboot, err := Asset(hddBootFile)
	if err != nil {
		log.Logf("using default content for %s", hddBootFile)
		hddboot = []byte(defaultHdd)
	}
	bd := bootData{
		Root_uuid:  root_uuid,
		Extra_opts: extra_opts,
		Kernel:     strs.BootKernel(),
	}
	out, err := bd.processTemplate(hddboot, hddBootFile)
	if err != nil {
		panic(fmt.Sprintf("error executing template for %s: %s", hddBootFile, err))
	}
	return fixupGrub(out)
}

const (
	defaultFallback = `timeout 0
default 0

title Recovery
errorcheck off
find --set-root /{{.FallFile}}
if not %@root:~5,1%==0 && root (hd0,0)
errorcheck on
kernel /{{.Kernel}} quiet
`
	defaultHdd = `timeout 0
default 0
fallback 1

title Normal
find --set-root /{{.Kernel}}
if not %@root:~5,1%==0 && root (hd0,0)
kernel /{{.Kernel}} real_root=UUID={{.Root_uuid}} quiet {{.Extra_opts}}

title Recovery
find --set-root /{{.Kernel}}
if not %@root:~5,1%==0 && root (hd0,0)
kernel /{{.Kernel}} quiet
`
	defaultMenu = `timeout 1
default 0
fallback 1

title Detecting active bootmedia
errorcheck off
root (hd0,0)
find --set-root /{{.HddFile}}
if not %@root:~5,1%==0 && root (hd0,0)
if exist /{{.HddFile}} && configfile /{{.HddFile}}
root (hd0,0)
find --set-root /{{.FallFile}}
if not %@root:~5,1%==0 && root (hd0,0)
configfile /{{.FallFile}}
errorcheck on

title Recovery
errorcheck off
root (hd0,0)
find --set-root /{{.Kernel}}
if not %@root:~5,1%==0 && root (hd0,0)
errorcheck on
kernel /{{.Kernel}} quiet
`
)

//Format boot partitions, write files to them. Ensures that kernel used is most recent of those available.
func WriteLegacyBootParts(disks []*Disk, target, recov *Filesystem, platform *appliance.Variant, extraOpts string) (bootParts []*Filesystem) {
	msg := "Writing boot partition..."
	if platform.HasRaid() {
		msg = "Writing redundant boot partitions..."
	}
	log.Msg(msg)

	if extraOpts != "" {
		extraOpts = " " + extraOpts
	}
	fn := platform.FamilyName()
	if fn == "qemu" {
		log.Log("enabling serial output for kernel")
		extraOpts += " console=ttyS0 console=tty0"
	}
	grubConf := finalizeGrubConf(target.Fsid(), extraOpts)
	useLatestKernel(target, recov)

	bootTgt := func(i int) int {
		if i == 1 {
			return 2
		}
		return 1
	}

	for i, d := range disks {
		b := new(Filesystem)
		bootParts = append(bootParts, b)
		bdev := fmt.Sprintf("%s%d", d.identifier, bootTgt(d.target))
		b.blkdev = fp.Join("/dev", bdev)
		b.mountPoint = fp.Join("/mnt", bdev)
		b.mountOpts = "noauto,relatime,noexec"
		b.mountType = "ext3" //grub4dos fails with ext4
		if platform.SSD() {
			b.mountOpts += ",discard"
		}
		_ = b.Format(fmt.Sprintf("boot%d", i))
		b.Mount()

		log.Msgf("Writing redundant boot files on %s", bdev)
		if err := ioutil.WriteFile(fp.Join(b.Path(), hddBootFile), grubConf, 0644); err != nil {
			log.Logf("write grub conf: %s", err)
		}
		copy2boot(strs.BootKernel(), b, target)
		//grub on hdds
		log.Msgf("Writing redundant boot loader on %s", bdev)
		b.InstallGrub4Dos()
	}
	return
}
