// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package disk

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	fp "path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/purecloudlabs/gprovision/pkg/appliance"
	"github.com/purecloudlabs/gprovision/pkg/appliance/altIdent"
	"github.com/purecloudlabs/gprovision/pkg/common/strs"
	futil "github.com/purecloudlabs/gprovision/pkg/fileutil"
	"github.com/purecloudlabs/gprovision/pkg/log"
)

// cause bindata.go to be generated from files in the data dir
//go:generate ../../../bin/go-bindata -prefix=../../../proprietary/data/disk -pkg=$GOPACKAGE ../../../proprietary/data/disk

//modified version of io.Copy(). slower, but allows progress reporting
func IOCopy(dst io.Writer, src io.Reader, progressFunc func(int64)) (written int64, err error) {
	buf := make([]byte, 32*1024)
	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[0:nr])
			if nw > 0 {
				written += int64(nw)
				progressFunc(written)
			}
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}
		if er == io.EOF {
			break
		}
		if er != nil {
			err = er
			break
		}
	}
	return written, err
}

//run mdadm to create and start array
func CreateArray(disks []*Disk, host string, platform *appliance.Variant) (md *Filesystem) {
	md = new(Filesystem)
	var parts []string
	for _, d := range disks {
		part := fmt.Sprintf("/dev/%s%d", d.identifier, d.target)
		parts = append(parts, part)
	}
	md.blkdev = "/dev/md0"
	md.mountPoint = "/mnt/md0"
	md.mountOpts = "auto,relatime"
	if platform.SSD() {
		md.mountOpts += ",discard"
	}
	rl := platform.RaidLevel()
	rlStr := fmt.Sprintf("raid%d", rl)
	log.Msgf("Creating %s array %s", rlStr, md.blkdev)
	mdadm := exec.Command("mdadm", "--create", md.blkdev, "--homehost", host, "-l", rlStr, "-n",
		fmt.Sprintf("%d", len(parts)))
	mdadm.Args = append(mdadm.Args, parts...) //there's a limit on the number of args to exec.Command?!

	mdadm.Stdin = strings.NewReader("y\n") //needs to see 'y' and newline on stdin
	out, err := mdadm.CombinedOutput()
	log.Log(string(out))
	if err != nil {
		log.Fatalf("error creating array")
	}
	return
}

//only for non-raid platforms
func (d *Disk) CreateNonArray(platform *appliance.Variant) (fs *Filesystem) {
	fs = new(Filesystem)
	fs.blkdev = fmt.Sprintf("/dev/%s%d", d.identifier, d.target)
	fs.mountPoint = fmt.Sprintf("/mnt/%s%d", d.identifier, d.target)
	fs.mountOpts = "auto,relatime"
	if platform.SSD() {
		fs.mountOpts += ",discard"
	}
	return
}

const K_OVERRIDE = "KERNEL_OVERRIDE"

// check the kernel version in the image and on recovery media, update whichever's older
func useLatestKernel(target, recov *Filesystem) {
	imgKernel := fp.Join(target.Path(), "boot", strs.BootKernel())
	recoveryKernel := fp.Join(recov.Path(), strs.BootKernel())
	override := os.Getenv(K_OVERRIDE)
	if override != "recovery" {
		newer := compareKernelVersions(imgKernel, recoveryKernel)
		if newer == "" {
			log.Logf("Kernels are from same build, not copying")
			return
		}
		if newer == imgKernel {
			//a new image was downloaded, and the recovery kernel wasn't overwritten
			log.Logf("image kernel is newer, updating recovery")
			//                          src, dest
			err := futil.CopyFile(imgKernel, recoveryKernel, 0)
			if err != nil {
				log.Logf("copying %s to %s: error %s", imgKernel, recoveryKernel, err)
			}
			return
		}
	}
	//unlikely
	log.Logf("recovery kernel is newer, updating image")
	err := futil.CopyFile(recoveryKernel, imgKernel, 0)
	if err != nil {
		log.Logf("copying %s to %s: error %s", recoveryKernel, imgKernel, err)
	}

}

//returns path of kernel with highest build number
func compareKernelVersions(kernel1, kernel2 string) (newest string) {
	ver1, success := getKBuild(kernel1)
	if !success {
		//getKVer already logged the error
		return kernel2
	}
	ver2, success := getKBuild(kernel2)
	if !success {
		return kernel1
	}
	if ver2 == ver1 {
		return ""
	}
	if ver2 < ver1 {
		return kernel1
	}
	return kernel2
}

//get kernel build number for kernel at 'kpath'.
func getKBuild(kpath string) (ver uint64, success bool) {
	//does file exist?
	_, err := os.Stat(kpath)
	if err != nil {
		log.Logf("error %s stat'ing %s", err, kpath)
		return
	}
	//run 'file' on kernel
	//returns something like /path/to/norm_boot: Linux kernel x86 boot executable bzImage, version x.y.z-norm_boot (user@host) #123 SMP date, RO-rootFS, swap_dev 0x11, Normal VGA
	out, err := exec.Command("file", kpath).CombinedOutput()
	if err != nil {
		log.Logf("err %s running 'file' on %s\noutput: %s", err, kpath, string(out))
		return
	}
	return kBuildNum(string(out), kpath)
}

//extract build number from 'file' output. split out for testability.
func kBuildNum(out, kpath string) (ver uint64, success bool) {
	ostr := strings.TrimSpace(strings.TrimPrefix(out, kpath+":"))

	//does file think it's a kernel?
	isKer := strings.Contains(ostr, "Linux kernel x86 boot executable")
	if !isKer {
		log.Logf("%s: not a kernel according to file (%s)", kpath, ostr)
		return
	}

	//extract version
	// blah blah #123 blah blah
	split := strings.Split(ostr, "#")
	if len(split) != 2 {
		log.Logf("error parsing file output %s for %s", ostr, kpath)
		return
	}
	//123 blah blah
	split2 := strings.SplitN(split[1], " ", 2)
	if len(split2) != 2 {
		log.Logf("error parsing file output %s for %s", ostr, kpath)
		return
	}
	ver, err := strconv.ParseUint(split2[0], 10, 64)
	if err != nil {
		log.Logf("error %s parsing version of %s (%s)", err, kpath, ostr)
		return
	}
	success = true
	return
}

/*
Assume recovery volume will have label imaging.RecVolName(), and read a file 'platform'
from the root dir. Return its contents, which will be the platform code name.
Use when normal platform ident fails. Volume is not left mounted.
*/
func PlatIdentFromRecovery() (string, error) {
	log.Logf("Failed to identify platform, trying file on recovery volume")
	path := "/dev/disk/by-label/" + strs.RecVolName()
	log.Msgf("Waiting for recovery to appear...")
	if !futil.WaitFor(path, 10*time.Second) {
		return "", fmt.Errorf("Failed to locate recovery volume by label")
	}
	dev, err := fp.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("Error identifying recovery volume: %s ", err)
	}
	fsType := "ntfs,vfat,ext4"
	fsOpts := "ro"
	fs := ExistingFs(dev, fsType, fsOpts, false)
	mp := "/mnt/r"
	err = os.MkdirAll(mp, 0755)
	if err != nil {
		log.Logf("Creating %s: %s", mp, err)
	}
	fs.SetMountpoint(mp)
	fs.Mount()
	content := altIdent.Read(mp)
	fs.Umount()
	if content == "" {
		return "", fmt.Errorf("Unknown platform")
	}
	log.Logf("Platform identity: %s", content)
	return content, nil
}
