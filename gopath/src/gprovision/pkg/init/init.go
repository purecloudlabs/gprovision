// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

// Package init implements early userspace logic in go, replacing the
// functionality of initramfs's /init. It does the normal things such as
// decoding
//   real_root=UUID=nnnnnn
// but also checks that the unit can/should boot in normal mode. It will
// display a menu on the attached LCD. This menu allows the user to initiate
// factory restore to the latest image, to a particular image, etc.
//
// Impacted by build tags mfg, release, erase_integ.
package init

import (
	"fmt"
	"gprovision/pkg/common/strs"
	"gprovision/pkg/hw/udev"
	"gprovision/pkg/log"
	"gprovision/pkg/log/flags"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/u-root/u-root/pkg/mount"
	"golang.org/x/sys/unix"
)

var verbose bool

//Calling Init() is equivalent to running the old /init shell script.
func Init() {
	log.SetPrefix("init")
	log.Log("init starting...")
	log.AdaptStdlog(nil, 0, true) //u-root uses the std log pkg

	handleEnvVars()
	//do we need udev to talk to a lcd? any sys files missing that way?
	//if we're not running as pid 1, skip some setup that should already be done
	if os.Getpid() == 1 {
		CreateDirs()
		EarlyMounts()
		/*
			we do not need to fiddle with pointing stdio at /dev/console, as the
			kernel does that whenever the initramfs contains /dev/console. buildroot's
			initramfs contains a /dev/console set up correctly - character special file,
			with correct major/minor numbers of 5,1.
		*/

		LdConfig()
		QuietPrintk()
		BBSymlinks()
	}
	handleEnvVarsPt2()
	uptime()
	var uproc *os.Process
	if os.Getpid() == 1 {
		uproc, _ = udev.Start()
	}
	//depending on build tags, this will be mfg or normal boot
	stage2(uproc)
}

func uptime() {
	uptime, err := ioutil.ReadFile("/proc/uptime")
	if err != nil {
		return
	}
	times := strings.Split(string(uptime), " ")
	//2nd element is apparently idle time, while first is total
	if len(times) != 2 {
		return
	}
	log.Logf("entered init at t+%s", times[0])
}

//translate something like        UUID=ed2d36e3-a3d9-408c-9255-897a010a783b
//into a path, i.e. '/dev/disk/by-uuid/ed2d36e3-a3d9-408c-9255-897a010a783b'
func getRoot(rr string) string {
	elements := strings.Split(rr, "=")
	if len(elements) == 2 {
		rootIdType := strings.ToLower(elements[0])
		if validIdType(rootIdType) {
			rootIdent := elements[1]
			return fmt.Sprintf("/dev/disk/by-%s/%s", rootIdType, rootIdent)
		}
	}
	fallback := "/dev/disk/by-label/" + strs.PriVolName()
	log.Logf("invalid value for real_root, falling back to %s", fallback)
	return fallback
}

func validIdType(id string) bool {
	for _, t := range []string{"id", "label", "partlabel", "partuuid", "path", "uuid"} {
		if id == t {
			return true
		}
	}
	return false
}

//set up busybox symlinks
func BBSymlinks() {
	log.Cmd(exec.Command("/bin/busybox", "--install", "-s"))
}

//update ld.so.cache
func LdConfig() {
	log.Cmd(exec.Command("/sbin/ldconfig"))
}

//prevent unimportant kernel messages from spamming the console
func QuietPrintk() {
	fname := "/proc/sys/kernel/printk"
	f, err := os.OpenFile(fname, os.O_TRUNC, 0600)
	if err != nil {
		log.Logf("silencing printk: %s", err)
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "1")
}

func CreateDirs() {
	for _, d := range []string{
		"/etc",
		"/etc/udev",
		"/etc/udev/hwdb.d",
		"/etc/udev/rules.d",
		"/dev",
		"/mnt",
		"/proc",
		"/run",
		"/sys",
		"/temp",
		"/tmp",
		"/var",
	} {
		err := os.Mkdir(d, 0777)
		if err != nil {
			if os.IsExist(err) {
				//already exists - fine as long as it's a dir
				fi, _ := os.Stat(d)
				if !fi.IsDir() {
					log.Logf("tried to create dir %s but something else exists with that name: %s", d, fi)
				}
			} else {
				log.Logf("error %s creating dir %s", err, d)
			}
		}
	}
}

type emount struct {
	fstype, dev, path, data string
	flags                   uintptr
}

var emounts []emount

func init() {
	emounts = []emount{
		{fstype: "sysfs", dev: "sysfs", path: "/sys", flags: unix.MS_NODEV | unix.MS_NOEXEC | unix.MS_NOSUID},
		{fstype: "devtmpfs", dev: "devtmpfs", path: "/dev"},
		{fstype: "proc", dev: "proc", path: "/proc", flags: unix.MS_NODEV | unix.MS_NOEXEC | unix.MS_NOSUID},
		{path: "/", data: "remount,rw", flags: unix.MS_REMOUNT /*| ^unix.MS_RDONLY*/},
	}
}

//create /sys, /dev, /proc (etc) mounts
func EarlyMounts() {
	for _, m := range emounts {
		err := mount.Mount(m.dev, m.path, m.fstype, m.data, m.flags)
		if err != nil {
			log.Logf("error %s mounting %s", err, m.path)
		}
	}
}

//unmount everything mounted by EarlyMounts
func EarlyUmounts() {
	for _, u := range emounts {
		if strings.Contains(u.data, "remount") {
			//skip remounts such as for /
			continue
		}
		err := mount.Unmount(u.path, false, true)
		if err != nil {
			log.Logf("error %s unmounting %s", err)
		}
	}
}

//boot delay - used to display menu and search for volumes
var (
	graceTime = 10 * time.Second
)

func commonEnvVars() {
	if os.Getenv(strs.VerboseEnv()) != "" {
		log.AddConsoleLog(flags.NA)
		verbose = true
	}
	if os.Getenv(strs.IntegEnv()) != "" {
		graceTime = 4 * time.Second
	}
	os.Setenv("PATH", "/sbin:/bin:/usr/bin:/usr/sbin")
}

//Loads an encryption key into the kernel keyring
type KeyLoader interface {
	LoadKey()
}

var fsKey KeyLoader
