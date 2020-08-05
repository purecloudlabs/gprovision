// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package recovery

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	fp "path/filepath"
	"regexp"
	"strings"

	"github.com/purecloudlabs/gprovision/pkg/common/stash"
	"github.com/purecloudlabs/gprovision/pkg/common/strs"
	dt "github.com/purecloudlabs/gprovision/pkg/disktag"
	futil "github.com/purecloudlabs/gprovision/pkg/fileutil"
	"github.com/purecloudlabs/gprovision/pkg/log"
)

// Hostify converts a string, typically the device serial number, into a string
// that is safe for use as a hostname. It adds strs.HostPrefix() and replaces
// some characters, such that the hostname matches the following regex:
// [a-z0-9][a-z0-9-]*[a-z0-9]
func Hostify(id string) string {
	hostify0 := func(r rune) rune {
		switch {
		case r >= 'A' && r <= 'Z':
			return 'a' - 'A' + r
		case r >= 'a' && r <= 'z':
			return r
		case r >= '0' && r <= '9':
			return r
		}
		return '-'
	}

	hn := strs.HostPrefix() + strings.Map(hostify0, id)
	if len(hn) == 0 {
		hn = "badhost"
	}
	if hn[len(hn)-1] == '-' {
		hn = hn[:len(hn)-1] + "0"
	}
	return hn
}

// TODO how to set the domain name? shows as unknown_domain now

//Set hostname, machine-id, password. func takes name from systemd-firstboot, but
//doesn't use that because it is completely broken.
func Firstboot(root, serial, hostName string) {
	futil.MkdirOwned(root, fp.Join("var", "log", "journal"), "root", "systemd-journal", 2755)

	//time zone is set to UTC
	hostInfo(root, hostName, serial)
	adminUser(root)

	// write disktag
	dt.Write(root)

	//write the system serial in config dir
	cfgDir := fp.Join(root, strs.ConfDir())
	err := os.MkdirAll(cfgDir, 0755)
	if err != nil {
		log.Logf("Error creating config dir: %s", err)
	}
	err = ioutil.WriteFile(fp.Join(cfgDir, "serial"), []byte(serial), 0644)
	if err != nil {
		log.Logf("Error writing system serial: %s", err)
	}
}

//create admin user acct
func adminUser(root string) {
	usr := "admin"
	pw, _ := stash.ReadOSPass()
	if len(pw) > 0 {
		/* centos 7 PAM config doesn't allow use of chpasswd. could provide our own
		 * file, but a simpler solution is to use busybox's chpasswd. originally it
		 * wasn't used because it lacked -R. however, 'chroot /path chpasswd' is an
		 * easy and functional workaround. it's unclear how many password algos are
		 * supported by busybox impl, but the one c7 uses is among those supported.
		 */
		chpw := exec.Command("busybox", "chroot", root, "chpasswd")
		chpw.Stdin = strings.NewReader(fmt.Sprintf("%s:%s\n", usr, pw))
		out, err := chpw.CombinedOutput()
		if err != nil {
			log.Msg("auth config issue")
			log.Log(fmt.Sprintln(chpw.Args, ":\nerror", err.Error(), "output", string(out)))
		}
	} else {
		log.Fatalf("stasher issue")
	}
}

//update /etc/hosts, host name, machine id, time zone
func hostInfo(root, hostName, serial string) {
	log.Msg("Hostname is " + hostName)
	host, err := os.Create(root + "/etc/hostname")
	if err == nil {
		defer host.Close()
		if _, err := host.Write([]byte(hostName + "\n")); err != nil {
			log.Logf("writing etc/hosts: %s", err)
		}
	} else {
		log.Log(fmt.Sprintf("cannot write /etc/hostname: %s\n", err))
	}

	//update etc/hosts
	localhost := "127.0.0.1   " + hostName + " localhost"
	blocalhost := []byte("\n" + localhost + "\n")
	re := regexp.MustCompile("127.0.0.1.*localhost")
	hosts, err := ioutil.ReadFile(root + "/etc/hosts")
	if err == nil {
		if re.Match(hosts) {
			hosts = re.ReplaceAllLiteral(hosts, []byte(localhost))
		} else {
			hosts = append(hosts, blocalhost...)
		}
	} else {
		if !os.IsNotExist(err) { //don't complain if it simply doesn't exist
			log.Log(fmt.Sprintf("error %s reading etc/hosts (or no match)", err))
		}
		hosts = blocalhost
	}
	err = ioutil.WriteFile(root+"/etc/hosts", hosts, 0644)
	if err != nil {
		log.Log(fmt.Sprintf("error %s writing etc/hosts", err))
	}

	//generate machine id from sha1 of S/N, doubled
	idf, err := os.Create(root + "/etc/machine-id")
	if err == nil {
		defer idf.Close()
		mid := sha1.Sum([]byte(serial + serial))
		fmt.Fprintf(idf, "%016x\n", mid)
	} else {
		log.Log(fmt.Sprintln("cannot write /etc/machine-id:", err.Error()))
	}
	localtime := fp.Join(root, "/etc/localtime")
	err = os.Remove(localtime)
	if err != nil {
		log.Logf("error %s removing old tz link", err)
	}
	err = os.Symlink("/usr/share/zoneinfo/Etc/UTC", localtime)
	if err != nil {
		log.Logf("error %s creating tz link", err)
	}
}

// Check bios raid setting on intel board, change if necessary.  Changing
// requires password.
//
// NOTE, syscfg always returns 0 (success and failure)!
//
// FIXME grub4dos can't boot if fakeraid remains enabled. Must be able to go
// back to windows in that case.
func CheckBios(tool string, update, reboot bool) (fakeraid bool) {
	if tool == "" {
		return false
	}
	execErr := fmt.Errorf("%s execution error", tool)
	successRead := []byte("Current Value")
	successSet := []byte("Successfully Completed")
	section := "Mass Storage Controller Configuration"
	item := "AHCI Capable SATA Controller"
	syscfgRead := exec.Command(tool, "/d", "BIOSSETTINGS", "group", section, item)
	out, err := syscfgRead.Output()
	if !bytes.Contains(out, successRead) {
		err = execErr
	}
	if err != nil {
		log.Logln(syscfgRead.Args, ":\nerror", err, "\noutput", string(out))
		errStr := "Cannot read setting from BIOS via syscfg"
		if reboot {
			log.Fatalf(errStr)
		} else {
			log.Log(errStr)
		}
	}
	if bytes.Contains(out, []byte("Current Value : AHCI")) {
		log.Log("BIOS RAID disabled")
		fakeraid = false
	} else {
		if !update {
			log.Log("BIOS RAID enabled, will disable later")
		}
		fakeraid = true
	}
	if update && fakeraid {
		bp, _ := stash.ReadBiosPass()
		syscfgSet := exec.Command(tool, "/bcs", bp, item, "02")
		out, err := syscfgSet.CombinedOutput()
		if err == nil && !bytes.Contains(out, successSet) {
			err = execErr
		}
		if err != nil {
			log.Logln(syscfgSet.Args, ":\nerror", err, "output", string(out))
		}
		//try a second time without password, just in case
		syscfgSet_nopw := exec.Command(tool, "/bcs", "", item, "02")
		out2, err2 := syscfgSet_nopw.CombinedOutput()
		if err2 == nil && !bytes.Contains(out2, successSet) {
			err2 = execErr
		}
		if err2 != nil {
			log.Logln(syscfgSet.Args, ":\nerror", err2, "output", string(out2))
		}
		if err != nil && err2 != nil {
			errStr := "Failed to reconfigure BIOS RAID via " + tool
			if reboot {
				log.Fatalf(errStr)
			} else {
				log.Msg(errStr)
			}
		}

		msg := "Changed BIOS RAID settings"
		if reboot {
			log.Fatalf(msg)
		} else {
			log.Msg(msg)
		}
	}
	return
}
