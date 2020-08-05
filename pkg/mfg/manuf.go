// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

//Package mfg is used to validate the hardware of freshly-assembled units and
//to image them.
package mfg

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	fp "path/filepath"
	"strings"

	"github.com/purecloudlabs/gprovision/pkg/appliance"
	"github.com/purecloudlabs/gprovision/pkg/appliance/altIdent"
	"github.com/purecloudlabs/gprovision/pkg/common"
	"github.com/purecloudlabs/gprovision/pkg/common/fr"
	"github.com/purecloudlabs/gprovision/pkg/common/rkeep"
	"github.com/purecloudlabs/gprovision/pkg/common/rlog"
	"github.com/purecloudlabs/gprovision/pkg/common/stash"
	"github.com/purecloudlabs/gprovision/pkg/common/strs"
	"github.com/purecloudlabs/gprovision/pkg/hw/block/partitioning"
	"github.com/purecloudlabs/gprovision/pkg/hw/cfa"
	"github.com/purecloudlabs/gprovision/pkg/hw/ipmi"
	"github.com/purecloudlabs/gprovision/pkg/hw/nic"
	"github.com/purecloudlabs/gprovision/pkg/hw/power"
	"github.com/purecloudlabs/gprovision/pkg/hw/udev"
	hk "github.com/purecloudlabs/gprovision/pkg/init/housekeeping"
	"github.com/purecloudlabs/gprovision/pkg/log"
	logflags "github.com/purecloudlabs/gprovision/pkg/log/flags"
	"github.com/purecloudlabs/gprovision/pkg/log/lcd"
	steps "github.com/purecloudlabs/gprovision/pkg/mfg/configStep"
	"github.com/purecloudlabs/gprovision/pkg/mfg/mdata"
	"github.com/purecloudlabs/gprovision/pkg/mfg/mfgflags"
	"github.com/purecloudlabs/gprovision/pkg/mfg/qa"
	"github.com/purecloudlabs/gprovision/pkg/net"
	"github.com/purecloudlabs/gprovision/pkg/recovery/disk"
)

var Platform *appliance.Variant

func Main() {
	log.AddConsoleLog(logflags.NA)
	log.SetFatalAction(MfgFatal)

	var err error
	if !udev.IsRunning() {
		_, err = udev.Start()
		if err == nil {
			//Start() logged the error, no need to log it ourselves
			log.Log("started udev")
		}
	}

	Platform, err = appliance.IdentifyWithFallback(mfgIdentFallback)
	if err != nil {
		log.Fatalf("identifying platform: %s", err)
	}

	//try setting up lcd unless we know there isn't one
	if Platform == nil || Platform.IsPrototype() || Platform.Lcd() != appliance.NoLCD {
		_, err := cfa.FindWithRetry()
		if err != nil {
			log.Logf("finding lcd: %s", err)
		}
	}
	if cfa.DefaultLcd != nil {
		err = lcd.AddLcdLog(logflags.EndUser)
		if err != nil {
			log.Logf("add lcd log: %s", err)
		}
	}
	//list of funcs that need to be executed pre-reboot
	hk.AddPrebootDefaults(disk.UnmountAll)

	mfgUrl := os.Getenv("mfgurl")
	if len(mfgUrl) == 0 {
		log.Fatalf("mfg url is missing")
	}

	defer power.RebootSuccess()
	Manuf(mfgUrl)
}

//Used to identify platform if appliance package fails. Useful for prototypes,
//if the prototype is similar enough to an existing variant.
func mfgIdentFallback() (string, error) {
	ident := os.Getenv("PROTO_IDENT")
	if ident != "" {
		return ident, nil
	}
	return disk.PlatIdentFromRecovery()
}

func Manuf(mfgDataUrl string) {
	log.SetFatalAction(MfgFatal)
	err := lcd.AddLcdLog(logflags.EndUser)
	if err != nil {
		log.Logf("add lcd log: %s", err)
	}
	if !mfgflags.Flag(mfgflags.SkipNet) {
		var diag []int
		var prefixes [][]byte
		if Platform != nil {
			diag = Platform.DiagPorts()
			prefixes = Platform.MACPrefixes()
		}
		success := net.EnableNetworkingSkipDIAG(diag, prefixes)
		if !success && (Platform == nil || Platform.IsPrototype()) {
			success = net.EnableNetworkingAny()
		}
		if !success {
			//no networking, we can't do much
			log.Fatalf("Network error")
		}
	}
	mfgData := mdata.Parse(mfgDataUrl)

	//match the kernel name, minus the extension
	logPfx := strs.MfgKernel()
	logPfx = strings.TrimSuffix(logPfx, fp.Ext(logPfx))
	log.SetPrefix(logPfx)

	if mfgData.ApplianceJsonUrl != "" {
		Platform = appliance.ReIdentify()
	}
	if !rlog.HaveRLogSetup() {
		log.Fatalf("RLoggerSetup is required but nil")
	}
	if Platform == nil {
		sn := "UNKNOWN_SN_" + log.Timestamp()
		err = rlog.Setup(mfgData.LogEndpoint, sn)
		if err != nil {
			log.Logf("add remote log: %s", err)
		}
		qa.Dump()
		log.Fatalf("failed to identify platform; logs at %s", sn)
	}
	err = rlog.Setup(mfgData.LogEndpoint, Platform.SerNum())
	if err != nil {
		log.Logf("add remote log: %s", err)
	}

	codeName := Platform.DeviceCodeName()
	if mfgData.ApplianceJsonUrl != "" {
		if mfgflags.Flag(mfgflags.ExternalJson) {
			log.Logf("External json identifies device as %s", codeName)
		} else {
			log.Fatalf("External json identifies device as %s", codeName)
		}
	}
	log.Msgf("mfg mode - %s,  SN: %s", codeName, Platform.SerNum())
	rkeep.ReportCodename(codeName)

	steps.CommonTemplateData.Serial = Platform.SerNum()
	cfgSteps := mfgData.CustomPlatCfgSteps.Find(codeName)
	if !cfgSteps.RunApplicable(steps.RunBeforeQA) {
		log.Fatalf("Failed to run a config step")
	}

	specs := mfgData.FindSpecs(codeName)
	specs.Validate(Platform)
	if mfgflags.Flag(mfgflags.StopAfterValidate) {
		fmt.Printf("stop after validation\n")
		os.Exit(0)
	}

	if !cfgSteps.RunApplicable(steps.RunAfterQA) {
		log.Fatalf("Failed to run a config step")
	}

	SetTimeFromServer()

	var recov *disk.Filesystem
	var noDelete bool
	var bootArgs string
	if mfgflags.Flag(mfgflags.NoRecov) {
		recov = disk.FindRecovery(Platform)
		if recov == nil {
			log.Fatalf("can't find recovery")
		}
		recov.Mount()
	} else {
		if os.Getenv(strs.ContinueLoggingEnv()) != "" {
			bootArgs = fmt.Sprintf("%s=%s", strs.LogEnv(), mfgData.LogEndpoint)
			noDelete = true
		}
		recov = disk.CreateRecovery(Platform, specs.Recovery.Size, bootArgs)
	}
	if recov == nil {
		log.Fatalf("can't find recovery")
	}
	if log.InStack(log.FileLogIdent) {
		log.Msg("already logging to file, not creating new file log")
	} else {
		_, err := log.AddFileLog(fp.Join(recov.Path(), strs.RecoveryLogDir()))
		if err != nil {
			log.Fatalf("cannot create log: %s", err)
		}
	}
	log.FlushMemLog()

	u := common.Unit{
		Rec:      recov,
		Platform: Platform,
	}
	stash.SetUnit(u)
	rkeep.SetUnit(u)
	fr.SetUnit(u)

	steps.CommonTemplateData.RecoveryDir = recov.Path()
	if !cfgSteps.RunApplicable(steps.RunBeforeImaging) {
		log.Fatalf("Failed to run a config step")
	}

	if !mfgflags.Flag(mfgflags.NoWrite) {
		mfgData.WriteFiles(recov)
	}

	if !cfgSteps.RunApplicable(steps.RunAfterImaging) {
		log.Fatalf("Failed to run a config step")
	}

	//must happen before manufacture(), as that causes DIAG port macs to disappear
	logMacs()
	//not affected by manufacture() but might as well do at same time...
	if Platform.HasIPMI() {
		ipmi.LogMacs()
	}

	if !cfgSteps.RunApplicable(steps.RunBeforeMfg) {
		log.Fatalf("Failed to run a config step")
	}

	if !mfgflags.Flag(mfgflags.NoMfg) {
		stash.Mfg()
	}

	if !cfgSteps.RunApplicable(steps.RunAfterMfg) {
		log.Fatalf("Failed to run a config step")
	}

	stash.HandleCredentials(cfgSteps)

	mfgData.FRConfig(recov, noDelete, bootArgs)

	if !mfgflags.Flag(mfgflags.NoWipe) {
		//wipe partition table on main disk(s), ensuring we really do boot into factory restore
		disks := disk.FindTargets(Platform)
		for _, d := range disks {
			if strings.HasPrefix(recov.Device(), d.Device()) {
				log.Fatalf("Fatal error: tried to wipe disk %s which contains %s (%s)", d.Device(), strs.RecVolName(), recov.Device())
			}
			log.Logf("wipe partition table on %s", d.Device())
			d.Zero(100, io.SeekStart)
		}
	}
	img := mfgData.FindImage()
	if img == nil {
		log.Fatalf("no image provided")
	}

	if appliance.IdentifiedViaFallback() {
		altIdent.Write(recov.Path(), Platform.DeviceCodeName())
	}

	//print out some info on the recovery disk's partitioning and fs
	parts := partitioning.List("/dev/" + recov.UnderlyingDevice())
	if len(parts) == 0 {
		log.Fatalf("failed to list partitions")
	}
	log.Logf("partitions on recovery:\n%s", parts)
	rdev, err := fp.EvalSymlinks(recov.Device())
	if err != nil {
		log.Logf("error evaluating symlinks in %s: %s", recov.Device(), err)
		rdev = recov.Device()
	}
	fsInfo, success := log.Cmd(exec.Command("file", "-s", rdev))
	if !success {
		log.Fatalf("file command failed")
	}
	log.Logf("recovery fs info:\n%s", fsInfo)
	if !strings.Contains(fsInfo, "ext3 filesystem") {
		log.Fatalf("not identified as ext3 filesystem")
	}

	//writes file (via logServer) to dir from which it'll be printed automatically
	qa.QASummary(img, specs, Platform, cfgSteps).Hardcopy()

	//reboot and allow normal factory restore to function
	log.Msg("Rebooting to factory restore...")
}

func logMacs() {
	nics := nic.SortedList(Platform.MACPrefixes())
	var macs []string
	for _, n := range nics {
		macs = append(macs, n.Mac().String())
	}
	rkeep.StoreMACs(macs)
}
