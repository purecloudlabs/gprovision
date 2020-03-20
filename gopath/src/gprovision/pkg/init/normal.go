// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

// +build !mfg

package init

import (
	"bufio"
	"gprovision/pkg/common/strs"
	"gprovision/pkg/erase"
	"gprovision/pkg/hw/block"
	"gprovision/pkg/hw/block/md"
	"gprovision/pkg/hw/cfa"
	"gprovision/pkg/hw/power"
	"gprovision/pkg/init/consts"
	"gprovision/pkg/init/progress"
	"gprovision/pkg/log"
	"gprovision/pkg/recovery"
	"io/ioutil"
	"os"
	fp "path/filepath"
	"strings"
	"time"

	"github.com/u-root/u-root/pkg/mount"
	"golang.org/x/sys/unix"
)

func stage2(uproc *os.Process) {
	real_root := os.Getenv("real_root")
	var must_erase bool
	se := os.Getenv(strs.EraseEnv())
	if se != "" {
		must_erase = true
	}

	switch {
	case must_erase:
		if verbose {
			log.Logf("mode: data erase")
		}
		erase.Main()
		power.RebootSuccess()
	case real_root != "":
		if verbose {
			log.Logf("mode: normal boot")
		}
		tryNormalBoot(real_root, uproc)
		if os.Getpid() != 1 {
			log.Logf("not init, exiting...")
			return
		}
	}

	if verbose {
		log.Logf("mode: recovery")
	}
	recovery.WithDefaults()
	power.FailReboot()
}

//look for emergency-mode files on inserted usb media
//this involves mounting, listing contents, and unmounting - but if file(s) are
//found, the device is left mounted and abs paths to the files are returned.
//Note that if multiple usb drives are inserted with files, only files on one
//(probably the first inserted) will be found.
func checkExtUsb() []string {
	err := os.Mkdir(consts.ExtDir, 0755)
	if err != nil && !os.IsExist(err) {
		log.Logf("error %s creating dir for ext usb", err)
	}
	prevChecked := func(checked []string, fs block.BlkInfo) bool {
		for _, id := range checked {
			if fs.UUID == id {
				return true
			}
		}
		return false
	}
	var checked []string
	fslist := block.GetFilesystems(block.BFiltNotRecovery, block.DFiltOnlyUsbParts)
	for _, fs := range fslist {
		if prevChecked(checked, fs) {
			continue
		}
		err = mount.Mount(fs.Device, consts.ExtDir, fs.FsType.String(), "", unix.MS_RDONLY)
		if err != nil {
			log.Logf("error %s searching %s for emergency-mode files", err, fs.Device)
			continue
		}
		errEncountered := false
		flist, err := fp.Glob(consts.ExtDir + "/" + strs.EmergPfx() + "*")
		if err != nil {
			log.Logf("error %s reading files on %s", err, fs.Device)
			errEncountered = true
		}
		if len(flist) > 0 {
			//leave it mounted
			return flist
		}
		err = mount.Unmount(consts.ExtDir, false, true)
		if err != nil {
			log.Logf("error %s unmounting %s", err, fs.Device)
			errEncountered = true
		}
		if !errEncountered && fs.UUID != "" {
			checked = append(checked, fs.UUID)
		}
	}
	return nil
}

//does not return unless factory restore (+reboot) is desired
func tryNormalBoot(real_root string, uproc *os.Process) {
	if cfa.DefaultLcd == nil {
		if verbose {
			log.Logf("search for lcd...")
		}
		_, err := cfa.FindWithRetry()
		if err != nil {
			log.Logf("error %s locating lcd", err)
		}
	}
	if verbose {
		log.Logf("md assemble")
	}
	md.AssembleScan()
	if verbose {
		log.Logf("root dev")
	}
	rootDev := getRoot(real_root)
	if verbose {
		log.Logf("user input")
		log.Logf("will search for root device %s", rootDev)
	}
	pressed, foundRoot, emergencyFiles := waitSearch(rootDev)
	testOpts()
	/*
		precedence:
			- user button press
			- emergency file
			- real root
		if nothing is found, return - which triggers factory restore
	*/
	if pressed {
		if verbose {
			log.Logf("boot menu")
		}
		bootMenu(rootDev, foundRoot, uproc)
	} else if len(emergencyFiles) != 0 {
		if verbose {
			log.Logf("e file")
		}

		_, _ = cfa.DefaultLcd.Msg("Emergency-mode file found. Processing...")
		recovery.WithEmergencyFile(emergencyFiles)
		power.RebootSuccess()
	} else if foundRoot {
		if verbose {
			log.Logf("switch root")
		}
		_, _ = cfa.DefaultLcd.Msg("Continuing normal boot...")
		switchroot(rootDev, uproc)
	}
}

func switchroot(rootDev string, uproc *os.Process) {
	//mount root on /newroot
	err := os.Mkdir(consts.NewRoot, 0755)
	if err != nil && !os.IsExist(err) {
		log.Logf("failed to create newroot: %s", err)
	}
	err = mount.Mount(rootDev, consts.NewRoot, "ext4", "", unix.MS_RDONLY)
	if err != nil {
		log.Logf("failed to mount newroot: %s", err)
		return
	}
	//check flag file
	_, err = os.Stat(fp.Join(consts.NewRoot, strs.FlagFile()))
	if err != nil {
		//if flag file doesn't exist, return (->factory restore)
		log.Logf("failed to stat %s: %s", strs.FlagFile(), err)
		err = mount.Unmount(consts.NewRoot, true, false)
		if err != nil {
			log.Logf("unmount error: %s", err)
		}
		return
	}
	if fsKey != nil {
		//load fs encryption key
		fsKey.LoadKey()
	}

	//spawn a process that draws spinner on lcd while systemd gets going
	progress.Fork()

	//give progress process a little time to start up
	time.Sleep(time.Second / 10)

	//kill udev
	if uproc != nil {
		err := uproc.Kill()
		if err != nil {
			log.Logf("error %s killing udevd", err)
		}
	}

	//SwitchRoot can't clean up if unexpected/extra mounts exist
	cleanMounts()

	err = mount.SwitchRoot(consts.NewRoot, consts.RealInit)
	if err != nil {
		log.Logf("switching root: %s", err)
		infos, err := ioutil.ReadDir(consts.NewRoot)
		log.Logf("newroot: err=%s", err)
		for i, fi := range infos {
			log.Logf("%d %s", i, fi.Name())
		}
		fi, err := os.Stat(fp.Join(consts.NewRoot, consts.RealInit))
		log.Logf("stat %s: err=%s fi=%#v", consts.RealInit, err, fi)
	}

	//should never get here
	power.RebootSuccess()
}

//unmount anything extraneous, otherwise SwitchRoot will error
func cleanMounts() {
	mounts, err := listMounts()
	if err != nil {
		log.Logf("listing mounts: %s", mounts)
	}
outer:
	for _, mnt := range mounts {
		for _, sfx := range []string{consts.NewRoot, "/dev/", "/proc/", "/sys/", "/run/"} {
			if strings.HasSuffix(mnt[1], sfx) || mnt[1] == "/" {
				//ignore it
				continue outer
			}
		}
		if err := mount.Unmount(mnt[1], false, true); err != nil {
			log.Logf("error %s umounting %s", err, mnt[1])
		}
	}
}

func listMounts() ([][]string, error) {
	f, err := os.Open("/proc/self/mounts")
	if err != nil {
		return nil, err
	}
	scanner := bufio.NewScanner(f)
	var mounts [][]string
	for scanner.Scan() {
		elems := strings.Split(scanner.Text(), " ")
		if len(elems) != 6 {
			log.Logf("failed to parse line %s with bad length %d", scanner.Text(), len(elems))
		}
		mounts = append(mounts, elems)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return mounts, nil
}

// Give user at least graceTime seconds to press button. Search for ext usb
// until button press or timeout. Search for real_root until button press or
// timeout.
func waitSearch(rootDev string) (pressed, foundRoot bool, emergencyFiles []string) {
	done := make(chan struct{})
	//look for emergency file
	go func() {
		for {
			emergencyFiles = checkExtUsb()
			if len(emergencyFiles) != 0 {
				return
			}
			select {
			case <-done:
				return
			case <-time.After(time.Second):
			}
		}
	}()
	//look for root
	go func() {
		for {
			select {
			case <-done:
				return
			case <-time.After(time.Second):
			}
			_, err := os.Stat(rootDev)
			if err == nil {
				foundRoot = true
				return
			}
		}
	}()
	//timeout signal
	go func() {
		time.Sleep(graceTime)
		close(done)
	}()
	var err error
	pressed, err = cfa.DefaultLcd.PressAnyKeyUntil("normal boot", time.Second, done)
	if err != nil && err != cfa.ENil {
		log.Logf("cfa error while waiting for key press: %s", err)
	}
	if pressed {
		select {
		case <-done:
			//do nothing
		default:
			//ensure that we've had a decent amount of time to find the root
			//volume, in case the user pressed a button quickly but then
			//chooses "normal boot"
			cfa.DefaultLcd.SpinnerUntil("Please wait...", time.Second, done)
		}
	}
	return pressed, foundRoot, emergencyFiles
}

var bootMenuItems []cfa.LcdTxt

const (
	Choice_resume cfa.Choice = iota
	Choice_poweroff
	_
	Choice_FR_latest
	Choice_FR_original
	Choice_FR_choose
	_
	Choice_erase
	CHOICE_LAST
)

func init() {
	bootMenuItems = make([]cfa.LcdTxt, CHOICE_LAST)
	bootMenuItems[Choice_resume] = cfa.LcdTxt("Resume normal boot")
	bootMenuItems[Choice_poweroff] = cfa.LcdTxt("Power off")
	//blank line
	bootMenuItems[Choice_FR_latest] = cfa.LcdTxt("Factory Restore to latest image")
	bootMenuItems[Choice_FR_original] = cfa.LcdTxt("Factory Restore to original image")
	bootMenuItems[Choice_FR_choose] = cfa.LcdTxt("Factory Restore (choose image)")
	bootMenuItems[Choice_erase] = cfa.LcdTxt("Data Erase")
}

func bootMenu(rootDev string, foundRoot bool, uproc *os.Process) {
	cfa.DefaultLcd.FlushEvents()
	var choice cfa.Choice
	var answer cfa.Answer
	for answer == cfa.ANSWER_NA {
		choice, answer = cfa.DefaultLcd.MenuWithConfirm("boot option", bootMenuItems, time.Minute*5, time.Minute, false)
		log.Logf("boot menu choice: %d, %d", choice, answer)
		switch choice {
		case cfa.CHOICE_CANCEL:
			fallthrough
		case cfa.CHOICE_NONE:
			fallthrough
		case Choice_resume:
			_, _ = cfa.DefaultLcd.Msg("Continuing normal boot...")
			switchroot(rootDev, uproc)
			return
		case Choice_poweroff:
			_, _ = cfa.DefaultLcd.Msg("Powering off...")
			power.Off()
		case Choice_FR_latest:
			_, _ = cfa.DefaultLcd.Msg("Factory restore with latest image...")
			recovery.WithDefaults()
		case Choice_FR_original:
			_, _ = cfa.DefaultLcd.Msg("Factory restore with original image...")
			recovery.WithImgOpt("oldest")
		case Choice_FR_choose:
			_, _ = cfa.DefaultLcd.Msg("Choose image to factory restore...")
			recovery.WithImgOpt("menu")
			//if this returns, go back to 1st menu
			answer = cfa.ANSWER_NA
		case Choice_erase:
			txt := "Data erase ERASES YOUR DATA and can take HOURS. A power failure or other interruption may render the device unusable. Choose NO unless absolutely certain."
			reconfirm := cfa.DefaultLcd.YesNo(cfa.LcdTxt(txt), time.Minute*5)
			if reconfirm != cfa.ANSWER_YES {
				answer = cfa.ANSWER_NA
				continue
			}
			erase.Main()
		default:
			if len(bootMenuItems[choice]) == 0 {
				_, _ = cfa.DefaultLcd.Msg("Invalid selection, continuing normal boot...")
				switchroot(rootDev, uproc)
			}
		}
	}
}
