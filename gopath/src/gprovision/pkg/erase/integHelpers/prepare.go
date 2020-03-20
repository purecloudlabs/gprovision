// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

// +build prep

//+ build !release

//prepare for erase integ test - create filesystem on recovery, write pattern on main volume
package main

import (
	"gprovision/pkg/appliance"
	"gprovision/pkg/hw/block"

	// "gprovision/pkg/hw/udev"
	ginit "gprovision/pkg/init"
	"gprovision/pkg/log"
	"gprovision/pkg/recovery/disk"
	"io"
	"os"
	"syscall"
)

func main() {
	log.AddConsoleLog(0)
	log.FlushMemLog()
	//FIXME cannot use Identify, no dmidecode
	// plat := appliance.Identify()

	// log.Log("recovery volume...")
	// recov := disk.CreateRecovery(plat, 10*block.GB, "")
	// err := os.Mkdir("/recov", 0755)
	// if err != nil {
	// 	log.Fatalf("mkdir: %s", err)
	// }
	// recov.SetMountpoint("/recov")
	// recov.Mount()
	ginit.CreateDirs()
	ginit.EarlyMounts()

	// _, err := udev.Start()
	// if err != nil {
	// 	log.Log(err)
	// }
	log.Log("primary/main volume...")
	//disks := disk.FindTargets(plat)
	devs := block.Devices()
	if len(devs) != 1 {
		log.Fatalf("need exactly 1 disk got %d", len(devs))
	}
	tgt := disk.DiskFromDev(devs[0])
	plat := appliance.Get("9p2k_dev")
	tgt.Partition(plat) //FIXME panic
	// primary := disks[0].CreateNonArray(plat)
	// primary.Format("purecloudedge")
	// err := os.Mkdir("/pri", 0755)
	// if err != nil {
	// 	log.Fatalf("mkdir: %s", err)
	// }
	// primary.SetMountpoint("/pri")
	// primary.Mount()
	err := writePatterns(tgt)
	if err != nil {
		log.Fatalf("error %s writing patterns", err)
	} else {
		log.Log("patterns written successfully. exiting.")
	}
}

//FIXME seqRunner
func writePatterns(d *disk.Disk) error {
	//write directly to block device
	//blkdev := d.Device()
	blk, err := os.OpenFile(d.Device(), os.O_WRONLY|syscall.O_DIRECT, 0)
	if err != nil {
		return err
	}
	defer blk.Close()
	siz := d.SizeBytes()
	offsets := offsetSeq(siz)
	//determine count/offsets
	for _, o := range offsets {
		err = writePat(blk, o)
		if err != nil {
			return err
		}
	}
	return nil
}
func writePat(f *os.File, offs int64) error {
	_, err := f.Seek(offs, io.SeekStart)
	if err != nil {
		//	log.Fatalf("seek: %s", err)
		return err
	}
	// buf := pat256()
	// const repeat = 1024
	// for i := 0; i < int(repeat); i++ {
	_, err = f.Write(pat1m(offs))
	return err
	// }
}
