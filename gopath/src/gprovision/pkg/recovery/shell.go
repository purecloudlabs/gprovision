// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package recovery

import (
	"bytes"
	"fmt"
	"gprovision/pkg/common/stash"
	"gprovision/pkg/log"
	"gprovision/pkg/recovery/disk"
	"os"
	"os/exec"
	"path/filepath"
)

//mount recovery, assemble/mount md0, drop to shell
func shell(recov *disk.Filesystem) {
	if recov != nil {
		mp, err := recov.MountErr()
		if err == nil {
			fmt.Printf("Recovery mounted at %s\n", mp)
		} else {
			log.Logf("failed to mount recovery: %s", err)
		}
		fmt.Printf("Array mounted at %s\n", readyArray())
	}
	if !log.LoggingToFile() {
		l, err := log.AddFileLog("/")
		if err != nil {
			log.Logf("logging to file: error %s", err)
		} else {
			fmt.Println("log written to", l)
		}
	}
	log.FlushMemLog()
	stash.RequestShellPassword()
	log.Msg("Dropping to shell...")
	log.Finalize()
	sh := exec.Command("setsid", "cttyhack", "sh")
	//attach to stdin/stdout/stderr
	sh.Stdin = os.Stdin
	sh.Stdout = os.Stdout
	sh.Stderr = os.Stderr

	if err := sh.Start(); err != nil {
		log.Logf("starting shell: %s", err)
	}
	if err := sh.Wait(); err != nil {
		log.Logf("running shell: %s", err)
	}
}

//FIXME support non-raid
func readyArray() string {
	mountpoint := "/mnt/md0"
	if err := os.MkdirAll("/dev/md", 0755); err != nil {
		log.Logf("creating /dev/md: %s", err)
	}
	mdadm := exec.Command("mdadm", "--assemble", "--scan")
	out, err := mdadm.CombinedOutput()
	log.Log(string(out))
	if err != nil {
		return "/dev/null"
	}
	// mdadm: /dev/md/0 has been started with ...
	s := bytes.Index(out, []byte("mdadm: /dev/md")) + 7
	e := s + bytes.IndexRune(out[s:], ' ')
	dev := string(out[s:e])
	_, err = os.Stat(dev)
	if os.IsNotExist(err) {
		log.Log(fmt.Sprintf("readyArray: %s does not exist (%s)", dev, err))
		//find md dev's with glob expr. if only 1, choose it
		MDlist, err := filepath.Glob("/dev/md*")
		if err == nil && len(MDlist) == 1 {
			dev = MDlist[0]
		} else if err == nil && len(MDlist) == 2 {
			if MDlist[0] == "/dev/md" {
				dev = MDlist[1]
			} else {
				dev = MDlist[0]
			}
		} else {
			log.Log(fmt.Sprintf("glob: %s for %s", err, MDlist))
			return "/dev/null"
		}
	}
	md := disk.ExistingExt4Fs(dev, false)
	md.SetMountpoint(mountpoint)
	//fmt.Printf("md mountpoint %s\n",mountpoint)
	path, err := md.MountErr()
	if err != nil {
		fmt.Printf("cannot mount array %s on %s: %s\n", dev, path, err)
	}
	return path
}
