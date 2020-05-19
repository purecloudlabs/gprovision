// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package disk

import (
	"bytes"
	"errors"
	"fmt"
	"gprovision/pkg/common"
	"gprovision/pkg/common/strs"
	futil "gprovision/pkg/fileutil"
	"gprovision/pkg/hw/block"
	"gprovision/pkg/log"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	fp "path/filepath"
	"strings"
	"time"

	"github.com/u-root/u-root/pkg/mount"
)

var mounted []string

type Filesystem struct {
	blkdev               string //absolute path, such as /dev/sda1
	formatted            bool
	isRecovery           bool
	mounted              bool
	mountType, mountOpts string
	mountPoint           string //where fs will normally be mounted
	currentMountPoint    string //where fs is currently mounted (if different from mountPoint, else empty)
	fsid                 string //unique identifier for fstab column 0
	label                string //name used during formatting
}

func ExistingExt4Fs(device string, mounted bool) (fs *Filesystem) {
	return ExistingFs(device, "ext4", "auto,relatime", mounted)
}

//ExistingFs creates a Filesystem struct corresponding to a fs that already
// exists. See also: Filesystem.AutoUnmount()
func ExistingFs(device, mntType, mntOpts string, mounted bool) (fs *Filesystem) {
	fs = new(Filesystem)
	fs.mountType = mntType
	fs.mountOpts = mntOpts
	fs.blkdev = device
	fs.formatted = true
	fs.mounted = mounted
	return
}

//Add fs to list for auto-unmount, if it isn't already. For use with ExistingFs().
func (fs *Filesystem) AutoUnmount() {
	if !fs.mounted {
		return
	}
	for _, m := range mounted {
		if m == fs.Path() {
			return
		}
	}
	mounted = append(mounted, fs.Path())
}

func TestFilesystem(dir string) (fs Filesystem) {
	fs.mounted = true
	fs.mountPoint = dir
	return
}

var NotRecoveryFS = errors.New("not a recovery fs")
var CantHandleThisFS = errors.New("Can't handle unknown format of recovery fs")

//recovery can have conflicting options for multiple filesystems. Remove any that don't make sense for this fs type.
func (fs *Filesystem) FixupRecoveryFS() (err error) {
	if !fs.isRecovery {
		return NotRecoveryFS
	}
	//special handling for 9p, as it can't be detected by blkdev / existingFsType
	if fs.mountType == "9p" && fs.blkdev == strs.RecVolName() {
		fs.mountOpts = removeOpts(fs.mountOpts, "uid=", "gid=", "user_id=", "group_id=")
		return
	}
	ft := block.DetermineFSType(fs.blkdev)
	if ft == block.FsNtfs {
		//ntfs-3g apparently doesn't support discard/trim
		fs.mountOpts = removeOpts(fs.mountOpts, "discard")
		fs.mountType = "ntfs-3g"
	} else if ft == block.FsExt4 {
		//ext4 fails when encountering these options
		fs.mountOpts = removeOpts(fs.mountOpts, "uid=", "gid=", "user_id=", "group_id=")
		fs.mountType = "ext4"
	} else {
		log.Log("unknown fs")
		err = CantHandleThisFS
	}
	return
}

func (fs Filesystem) FstabEntry(uid, gid string) (entry string) {
	pass := 2
	if fs.mountPoint == "/" {
		pass = 1
	}
	opts := strings.Replace(fs.mountOpts, "$u", uid, -1)
	opts = strings.Replace(opts, "$g", gid, -1)
	if len(fs.fsid) != 0 && !strings.Contains(fs.blkdev, "by-label") {
		entry = fmt.Sprintf("UUID=\"%s\" %s %s %s %d %d #%s\n", fs.fsid, fs.mountPoint, fs.mountType, opts, 0, pass, fs.blkdev)
	} else {
		entry = fmt.Sprintf("%s %s %s %s %d %d #%s\n", fs.blkdev, fs.mountPoint, fs.mountType, opts, 0, pass, fs.blkdev)
	}
	return
}

//sets the mount point of a fs, before writing fstab. if mounted, mount location is still retrievable via Path()
func (fs *Filesystem) SetMountpoint(pt string) {
	if fs.mounted && len(fs.mountPoint) > 0 {
		fs.currentMountPoint = fs.mountPoint
	}
	fs.mountPoint = pt
}

func (fs Filesystem) IsMounted() bool {
	return fs.mounted
}

//true if it appears that it would be possible to mount the fs - regardless of whether it is mounted at this time
func (fs Filesystem) Valid() bool {
	return len(fs.blkdev) != 0
}

func (fs Filesystem) Device() string {
	return fs.blkdev
}

//returns current mount point, or "/dev/null"
func (fs Filesystem) Path() string {
	if fs.mounted {
		if len(fs.currentMountPoint) > 0 {
			return fs.currentMountPoint
		}
		return fs.mountPoint
	}
	log.Logf("WARNING: fs.Path(): not mounted %#v", fs)
	return "/dev/null"
}

func (fs Filesystem) FstabMountpoint() string { return fs.mountPoint }

func (fs *Filesystem) Format(label string) (err error) {
	if fs.formatted {
		log.Logf("WARNING: we have already formatted %s (label %s), will not reformat with label %s/type %s", fs.blkdev, fs.label, label, fs.mountType)
		return nil
	}
	fs.label = label
	//wait for the block device to appear, apparently it isn't instantaneous
	//if zero() doesn't see the device it prints an alarming message
	found := futil.WaitFor(fs.blkdev, 5*time.Second)
	if !found {
		log.Logf("warning - device %s has not appeared", fs.blkdev)
	}
	zero(fs.blkdev, 1, io.SeekStart)
	if len(fs.mountType) == 0 {
		fs.mountType = "ext4"
	}
	log.Logf("formatting %s as %s, label %s", fs.blkdev, fs.mountType, label)
	var cmd string
	var args []string
	var expectUuid bool
	if fs.mountType == "vfat" {
		cmd = "mkdosfs"
		args = []string{"-n", label, fs.blkdev}
	} else {
		args = []string{"-L", label, "-m", "1", "-t", fs.mountType}
		if fs.mountType == "ext4" {
			//if it's ext4, make it possible use directory encryption
			args = append(args, "-O", "encrypt")
		}
		args = append(args, fs.blkdev)
		cmd = "mke2fs"
		expectUuid = true
	}
	mkfs := exec.Command(cmd, args...)
	out, err := mkfs.CombinedOutput()
	if err != nil {
		log.Logf("exec %v: err %s\n out\n%s\n", mkfs.Args, err, out)
		fs.mountType = ""
		return
	}
	/* could run tune2fs to set max mount count/interval. however, mkfs defaults to
	 * not do so; when errors are encountered fs will be marked as dirty anyway
	 */
	if expectUuid {
		var uu, nl int
		uu = bytes.Index(out, []byte("UUID: "))
		if uu >= 0 {
			nl = bytes.Index(out[uu:], []byte("\n"))
		}
		if nl < 0 || uu < 0 {
			log.Logf("exec %v: can't parse output\n%s", mkfs.Args, out)
			err = os.ErrInvalid
			return
		}
		nl += uu
		fs.fsid = string(out[uu+6 : nl])
	}
	fs.formatted = true
	return
}

func (fs Filesystem) Fsid() string {
	return fs.fsid
}
func (fs Filesystem) Label() string {
	return fs.label
}

func (fs *Filesystem) Mount() {
	_, err := fs.MountErr()
	if err != nil {
		dc, e := ioutil.ReadDir("/dev")
		if e != nil {
			log.Logf("ioutil.ReadDir error: %s", e)
		}
		for _, d := range dc {
			if strings.HasPrefix(d.Name(), fs.mountPoint[:2]) {
				log.Logf("mounting %s: found /dev/%s", fs.mountPoint, d.Name())
			}
		}
		log.Fatalf("error mounting %s: %s", fs.blkdev, err)
	}
}
func (fs *Filesystem) MountErr() (path string, err error) {
	path = fs.mountPoint
	if len(path) < 1 {
		err = fmt.Errorf("path too short!")
		return
	}
	if fs.mounted {
		return
	}
	err = os.MkdirAll(fs.mountPoint, 0700)
	if err != nil {
		log.Logln(err)
	}

	// we want nofail to be in fstab in some cases, but here we
	// need to know of failures - so don't pass it to mount
	opts := removeOpts(fs.mountOpts, "nofail", "auto", "uid=", "gid=", "user_id=", "group_id=")

	// Try u-root's Mount(). Not sure if it'll work on things like NTFS-3g
	// (FUSE), so if this reports an error try with the mount binary.
	err = mount.Mount(fs.blkdev, fs.mountPoint, fs.mountType, opts, 0)
	if err == nil {
		log.Logf("mount %s on %s", fs.blkdev, fs.mountPoint)
		fs.mounted = true
		mounted = append(mounted, fs.mountPoint)
		return
	}
	log.Logf("u-root mount failed with %s, trying binary...", err) //
	mnt := exec.Command("mount", fs.blkdev, fs.mountPoint, "-t", fs.mountType)
	if opts != "" {
		mnt.Args = append(mnt.Args, "-o", opts)
	}
	out, err := mnt.CombinedOutput()
	if err != nil {
		log.Logln(mnt.Args, "\nerror:", err.Error(), "\nout:", string(out))
		return "", err
	}
	fs.mounted = true
	mounted = append(mounted, fs.mountPoint)
	return
}

func (fs *Filesystem) Umount() {
	if !fs.mounted {
		log.Logf("umount: %s not mounted", fs.blkdev)
		return
	}
	err := mount.Unmount(fs.blkdev, false, true)
	if err != nil {
		log.Logf("umount %s: %s", fs.blkdev, err)
	} else {
		log.Logf("umount %s", fs.blkdev)
		fs.mounted = false
	}
}

//remove options from comma-separated list. if opt to remove ends with '=', match beginning of an item in opts
func removeOpts(opts string, removes ...string) (cleanOpts string) {
	//convert opts string to array
	arr := strings.Split(opts, ",")
	for _, o := range arr {
		skip := false
		for _, r := range removes {
			if r == o || (strings.HasSuffix(r, "=") && strings.HasPrefix(o, r)) {
				skip = true
				break
			}
		}
		if !skip {
			cleanOpts += o + ","
		}
	}
	return strings.Trim(cleanOpts, ",")
}

func (fs Filesystem) WriteFstab(uid, gid string, mounts ...common.FS) {
	fstab, err := os.Create(fp.Join(fs.Path(), "etc", "fstab"))
	if err != nil {
		log.Fatalf("cannot write fstab!")
	}
	defer fstab.Close()
	for _, entry := range mounts {
		if _, err = fstab.WriteString(entry.FstabEntry(uid, gid)); err != nil {
			log.Logf("write fstab: %s", err)
		}
		err = os.MkdirAll(fp.Join(fs.Path(), entry.FstabMountpoint()), 0755)
		if err != nil {
			log.Log(fmt.Sprintf("error creating mountpoint %s: %s", entry.FstabMountpoint(), err))
		}
	}
}

/* Find the identifier (e.g. sdb) of the physical device this filesystem is on.
   blkdev is probably a partition (subdevice). Try to find base device by
   removing chars one at a time and checking if a file with that name exists
   in /sys/block.
*/
func (fs *Filesystem) UnderlyingDevice() (dev string) {
	resolved, err := fp.EvalSymlinks(fs.blkdev)
	if err != nil {
		log.Logf("error resolving symlinks in %s, trying as-is", fs.blkdev)
		resolved = fs.blkdev
	}
	dev = fp.Base(resolved)
	for {
		// /sys/block only contains devices, while /sys/class/block contains partitions as well - ??
		if _, err := os.Stat(fp.Join("/sys", "block", dev)); !os.IsNotExist(err) {
			break
		}
		i := len(dev)
		if i < 2 {
			log.Logf("failed to find underlying device for partition %s", fs.blkdev)
			dev = ""
			break
		}
		dev = dev[:i-1]
	}
	return
}

//copy fname from source/boot to b1 and b2
func copy2boot(fname string, dest, source *Filesystem) {
	sPath := fp.Join(source.Path(), "boot", fname)
	dPath := fp.Join(dest.Path(), fname)
	err := futil.CopyFile(sPath, dPath, 0)
	if err != nil {
		log.Logf("copying %s to %s: error %s", sPath, dPath, err)
	}
}

func UnmountAll(_ bool) {
	log.Logf("Unmount all disks")
	for i := len(mounted) - 1; i >= 0; i-- {
		mnt := mounted[i]
		um := exec.Command("umount", "-lr", mnt)
		//FIXME error umount: can't forcibly umount ... : Invalid argument
		out, err := um.CombinedOutput()
		if err != nil {
			log.Log(fmt.Sprintf("umount %s error %s\n%s", mnt, err, string(out)))
		}
	}
	mounted = nil
}

func (fs Filesystem) SetOwnerAndPerms(uid, gid string) {
	if !fs.mounted {
		log.Logf("fs not mounted, cannot modify owner/perms")
		return
	}
	path := fs.Path()
	chownr := exec.Command("chown", "-R", uid+":"+gid, path)
	out, err := chownr.CombinedOutput()
	if err != nil {
		log.Logf("%s: err %s\noutput:\n%s", chownr.Args, err, out)
		return
	}
	chmodr := exec.Command("chmod", "-R", "a+rX", path)
	out, err = chmodr.CombinedOutput()
	if err != nil {
		log.Logf("%s: err %s\noutput:\n%s", chmodr.Args, err, out)
	}
}
