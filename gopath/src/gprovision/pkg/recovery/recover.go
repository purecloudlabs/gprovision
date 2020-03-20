// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package recovery

import (
	"fmt"
	"gprovision/pkg/appliance"
	"gprovision/pkg/common"
	"gprovision/pkg/common/fr"
	"gprovision/pkg/common/rkeep"
	"gprovision/pkg/common/stash"
	"gprovision/pkg/common/strs"
	dt "gprovision/pkg/disktag"
	futil "gprovision/pkg/fileutil"
	"gprovision/pkg/hw/cfa"
	"gprovision/pkg/hw/power"
	"gprovision/pkg/hw/udev"
	"gprovision/pkg/id"
	hk "gprovision/pkg/init/housekeeping"
	"gprovision/pkg/log"
	logflags "gprovision/pkg/log/flags"
	"gprovision/pkg/log/lcd"
	"gprovision/pkg/recovery/archive"
	"gprovision/pkg/recovery/disk"
	"gprovision/pkg/recovery/emode"
	"gprovision/pkg/recovery/history"
	netd "gprovision/pkg/systemd/networkd"
	"os"
	"os/exec"
	fp "path/filepath"
	"time"

	"github.com/u-root/u-root/pkg/mount"
)

var Platform *appliance.Variant

func WithEmergencyFile(efiles []string) { rmain(efiles, "") }
func WithImgOpt(opt string)             { rmain(nil, opt) }
func WithDefaults()                     { rmain(nil, "") }

var frModeEnv = "frmode"

func rmain(efiles []string, imgopt string) {
	if !udev.IsRunning() {
		_, _ = udev.Start()
		log.Log("started udev")
	}
	log.AddConsoleLog(logflags.EndUser)
	log.SetFatalAction(RecFatal)
	var err error
	Platform, err = appliance.IdentifyWithFallback(disk.PlatIdentFromRecovery)
	if err != nil {
		log.Fatalf("unknown platform: %s", err)
	}
	recov := disk.FindRecovery(Platform)
	hk.AddPrebootDefaults(disk.UnmountAll)
	_, err = cfa.FindWithRetry()
	if err != nil {
		log.Logf("lcd find failed: %s", err)
	}
	if cfa.DefaultLcd != nil {
		if err := lcd.AddLcdLog(logflags.EndUser); err != nil {
			log.Logf("add lcd log: %s", err)
		}
	}
	if recov == nil {
		log.Fatalf("failed to find recovery device")
	}
	u := common.Unit{
		Platform: Platform,
		Rec:      recov,
	}
	stash.SetUnit(u) //must be before any call to shell(), stash is needed for pw

	/* mode env var
	 *   -----
	 * default (recovery): do factory restore
	 * shell: mount recovery & md, drop to shell. password protected via ipmi password
	 */
	mode := os.Getenv(frModeEnv)
	if mode == "shell" {
		shell(recov)
	} else {
		noreboot := recovery(recov, u, efiles, imgopt)
		if noreboot {
			//Return to main menu rather than rebooting, likely because the
			//user canceled in image menu.
			return
		}
	}

	log.Msg("done, rebooting")
	power.RebootSuccess()
}

func recovery(recov *disk.Filesystem, u common.Unit, efiles []string, imgopt string) (noreboot bool) {
	log.Msg("start recovery: " + log.Timestamp())
	recov.Mount()
	rpath := recov.Path()

	serial := Platform.SerNum()

	log.Msg("Recovery media at " + rpath)
	log.SetPrefix(strs.FRLogPfx())
	if log.InStack(log.FileLogIdent) {
		log.Msg("already logging to file, not creating new file log")
	} else {
		if _, err := log.AddFileLog(fp.Join(rpath, strs.RecoveryLogDir())); err != nil {
			log.Logf("adding file log: %s", err)
		}
	}

	//history records which images have been tried and whether they failed
	history.SetRoot(rpath)
	hk.Preboots.Add(&hk.HkTask{
		Func: history.RebootHook,
		Name: "history",
	})

	dt.SetPlatform(Platform.DeviceCodeName())

	eJsons, emergencyImage := emode.CheckForEmergency(efiles)

	fr.SetUnit(u)
	err := fr.ReadRecoveryOr(eJsons)
	if err != nil {
		log.Logf("opening rjson: %s", err)
	}
	//sets up remote logging etc
	err = fr.Handle()
	if err != nil {
		log.Logf("handling fr data: %s", err)
	}
	log.FlushMemLog()

	rkeep.SetUnit(u) //with pblog, must call after rlog.Setup() / frd.Handle()

	log.Logf("system info: raid? %t, drives: %d, sn %s", Platform.HasRaid(), Platform.DataDisks(), serial)

	futil.ForcePathCase(rpath, "Image")

	log.Msg("waiting on update validation...")
	//don't bother doing this in the background, adds complexity without benefit
	updOk, userCancel := archive.FindValidUpd(emergencyImage, imgopt, fp.Join(rpath, "Image"), Platform.LowMemory())
	if userCancel {
		if imgopt == "" {
			//should never get here
			panic("user cancel with no imgopt")
		}
		//discard preboot items; the only one we need is UnmountAll
		hk.Preboots.Clear()
		disk.UnmountAll(false)
		return true //do not reboot
	}
	if !updOk {
		log.Fatalf("no valid update files found")
	}
	//FIXME refactor
	fakeRaid := false
	if Platform.HasRaid() {
		fakeRaid = CheckBios(Platform.BiosConfigTool(), false, false)
	}
	if fakeRaid {
		CheckBios(Platform.BiosConfigTool(), true, false)
	}

	disks := disk.FindTargets(Platform)
	if Platform.HasRaid() {
		paths := ""
		for _, d := range disks {
			paths += d.Device() + " "
		}
		log.Logf("RAID%d array using %s", Platform.RaidLevel(), paths)
	}

	for _, d := range disks {
		log.Msgf("partitioning %s", d.Device())
		err = d.Partition(Platform)
		if err != nil {
			log.Logf("partitioning %s: %s", d.Device(), err)
		}
	}
	hostName := Hostify(serial)
	var target *disk.Filesystem
	if Platform.HasRaid() {
		target = disk.CreateArray(disks, hostName, Platform)
	} else {
		if len(disks) != 1 {
			log.Logf("error - multiple disks identified on non-raid platform. disks[]=%#v", disks)
			log.Fatalf("unsupported configuration")
		}
		target = disks[0].CreateNonArray(Platform)
	}
	_ = target.Format(strs.PriVolName())
	target.Mount()

	log.Msg("Copying files...")
	archive.ApplyUpdate(target)
	if emergencyImage != "" {
		//unmount the usb drive the emergency image was on
		if err := mount.Unmount(fp.Dir(emergencyImage), false, false); err != nil {
			log.Logf("umount drive containing emergency image: %s", err)
		}
	}

	recov.SetMountpoint("/mnt/" + strs.RecVolName())
	target.SetMountpoint("/")

	bootArgs := fr.AdditionalBootArgs()
	bootParts := disk.MakeBootable(disks, target, recov, Platform, bootArgs)

	var parts []common.FS
	parts = append(parts, target, recov) //target (root) must be first in fstab
	for _, p := range bootParts {
		parts = append(parts, p)
	}

	//uid,gid are used when mounting the recovery key to ensure that our user can access it, since non-native fs types map to root by default.
	uid, gid := getUidGid(target)
	target.WriteFstab(uid, gid, parts...)

	writeNetworkConfig(recov, target)

	Firstboot(target.Path(), serial, hostName)

	//unmount ESP - otherwise, that dir's owner/perms can't be set
	for _, p := range bootParts {
		if p.Label() == "ESP" {
			p.Umount()
		}
	}
	recov.SetOwnerAndPerms(uid, gid)

	err = fr.Delete()
	if err != nil {
		log.Logf("deleting FRData: %s", err)
	}

	//create file that tells us normal boot is ok
	f, err := os.OpenFile(fp.Join(target.Path(), strs.FlagFile()), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		log.Logf("err %s creating flag file", err)
		log.Fatalf("failed to set normal boot flag")
	} else {
		f.Close()
	}

	//write platform info to disk
	writeFacts(target)

	log.Msg("recovery process complete")
	return false //reboot
}

func writeNetworkConfig(recov, target common.Pather) {
	restoreNetworkConfig(recov, target)

	//write defaults regardless of whether an old config was restored
	dir := "/etc/systemd/network-defaults"
	futil.MkdirOwned(target.Path(), dir, "admin", "wheel", 0755)
	defaults := netd.Defaults(Platform)
	netd.Write(defaults, target.Path()+dir)
}

//looks for tarball, extracts if found
func restoreNetworkConfig(recov, target common.Pather) {
	defaultCfgErr := func() { log.Msgf("network config error - using defaults") }
	netConf := fp.Join(recov.Path(), strs.RecoveryLogDir(), "netd.tar")
	_, err := os.Stat(netConf)
	if os.IsNotExist(err) {
		log.Msgf("no network config to restore")
		return
	}
	if err != nil {
		defaultCfgErr()
		log.Logf("stat netd.tar failed, err=%s", err)
		return
	}
	if fr.IgnoreNetworkConfig() {
		log.Msg("recovery opts: ignoring preserved network config")
		err = os.Remove(netConf)
		if err != nil {
			log.Logf("error while deleting %s: %s", netConf, err)
		}
		return
	}
	netDir := target.Path() + "/etc/systemd/network"
	err = os.RemoveAll(netDir)
	if err != nil {
		log.Logf("removing %s: %s", netDir, err)
	}
	err = os.MkdirAll(netDir, 0755)
	if err != nil {
		defaultCfgErr()
		log.Logf("MkdirAll failed, err=%s", err)
		return
	}
	untar := exec.Command("tar", "xf", netConf, "-C", netDir)
	var out []byte
	out, err = untar.CombinedOutput()
	if err != nil {
		defaultCfgErr()
		log.Logln(untar.Args)
		log.Logf("untar failed - err: %s\nout:\n%s\n", err, out)
		return
	}
	var f *os.File
	flag := netDir + "/.config-preserved" //tells ansible to leave config alone
	f, err = os.Create(flag)
	if err == nil {
		f.Close()
		log.Msgf("network config restored")
		err = os.Remove(netConf)
		if err != nil {
			log.Logf("error while deleting %s: %s", netConf, err)
		}
		time.Sleep(time.Second)
	}
}

func getUidGid(target *disk.Filesystem) (uid, gid string) {
	uid = "1001" //this initial value (a guess) is used in the unlikely event that we can't read /etc/passwd, /etc/group
	gid = "1001"
	u, err := id.GetUID(target.Path(), "admin")
	if err != nil {
		log.Logln(err)
	}
	if u >= 0 {
		uid = fmt.Sprintf("%d", u)
	}
	g, err := id.GetGID(target.Path(), "admin")
	if err != nil {
		log.Logln(err)
	}
	if g >= 0 {
		gid = fmt.Sprintf("%d", g)
	}
	return
}

func writeFacts(tgt *disk.Filesystem) {
	ltype := Platform.Lcd()
	if Platform.IsPrototype() && ltype == appliance.NoLCD && cfa.DefaultLcd != nil {
		switch cfa.DefaultLcd.Model() {
		case cfa.Cfa631:
			ltype = appliance.Cfa631
		case cfa.Cfa635:
			ltype = appliance.Cfa635
		}
	}
	Platform.WriteOut(tgt.Path(), ltype)
}
