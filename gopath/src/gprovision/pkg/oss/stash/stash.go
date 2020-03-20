// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

// Package stash - oss impl of stash
package stash

import (
	"errors"
	"fmt"
	"gprovision/pkg/common"
	"gprovision/pkg/common/stash"
	"gprovision/pkg/log"
	steps "gprovision/pkg/mfg/configStep"
	"gprovision/pkg/oss/pblog"
	"io/ioutil"
	"os"
	"os/exec"
	fp "path/filepath"
	"strings"
)

func UseImpl() {
	stash.SetImpl(&ostash{})
}

type ostash struct {
	u  common.Unit
	sd common.StashData
	cr common.Credentials
}

//set serial number, recovery volume, etc
func (os *ostash) SetUnit(u common.Unit) { os.u = u }

//called immediately after mfg data is parsed
func (os *ostash) SetData(sd common.StashData) { os.sd = sd }

// Determine unit credentials, store. Does not set IPMI/BIOS pw - that would
// require a mfg-specific OOB tool.
func (s *ostash) HandleCredentials(cfgSteps steps.ConfigSteps) {
	ep := s.sd.CredEP()
	if ep != "pblog" {
		log.Fatalf("ostash.HandleCredentials: unknown log type %q", ep)
	}
	pblg := log.FindInStack(pblog.LogIdent)
	if pblg == nil {
		log.Fatalf("no pblog - ?!")
	}
	log.Logf("ostash.HandleCredentials uses INSECURE password storage")
	pbl := pblg.(*pblog.Pbl)
	s.cr = pbl.GetCredentials(s.u.Platform.SerNum())

	steps.AddPWs(s.cr.BIOS, s.cr.IPMI, s.cr.OS)
	defer func() { steps.AddPWs("", "", "") }()

	cfgSteps.RunApplicable(steps.RunBeforePWSet)

	log.Logf("writing credentials to insecure.storage")
	data := []byte(fmt.Sprintf("%s\000%s\000%s", s.cr.BIOS, s.cr.IPMI, s.cr.OS))
	err := ioutil.WriteFile(fp.Join(s.u.Rec.Path(), "insecure.storage"), data, 0600)
	if err != nil {
		log.Fatalf("writing pws: %s", err)
	}

	cfgSteps.RunApplicable(steps.RunAfterPWSet)
}

//Stores other secrets.
func (s *ostash) Mfg() {
	for _, sf := range s.sd.StashFileList() {
		sf.UseIntermediateDir("/tmp")
		err := sf.GetWithRetry()
		if err != nil {
			log.Fatalf("failed to download mfg tarball: %s", err)
		}
		tarball := sf.GetIntermediate()
		defer os.Remove(tarball)
		work, err := ioutil.TempDir("", "mfg")
		if err != nil {
			log.Fatalf("mfg: creating temp dir: %s", err)
		}
		extract := exec.Command("tar", "xJf", tarball, "-C", work)
		_, success := log.Cmd(extract)
		if !success {
			log.Fatalf("failed to extract %s", sf.Basename())
		}
		//find exact name of the script
		glob := work + "/*.sh"
		matches, _ := fp.Glob(glob)
		if len(matches) != 1 {
			log.Fatalf("archive must contain exactly one file matching %s", fp.Base(glob))
		}
		exe := exec.Command(matches[0])
		out, err := exe.CombinedOutput()
		if err != nil {
			log.Fatalf("executing %#v: %s\noutput:\n%s", exe.Args, err, string(out))
		}
		log.Logf("output for %s:\n%s", sf.Basename(), string(out))
	}
}

func (s *ostash) loadPWs() error {
	data, err := ioutil.ReadFile(fp.Join(s.u.Rec.Path(), "insecure.storage"))
	if err != nil {
		return err
	}
	if len(data) < 10 {
		//10 is arbitrary. intended to detect corruption, not to ensure
		//pw strength
		return errors.New("pw data too short")
	}
	pws := strings.Split(string(data), "\000")
	if len(pws) != 3 {
		return errors.New("wrong number of items")
	}
	s.cr.BIOS = pws[0]
	s.cr.IPMI = pws[1]
	s.cr.OS = pws[2]
	return nil
}

//Returns OS Password.
func (s *ostash) ReadOSPass() (string, error) {
	if len(s.cr.OS) == 0 {
		err := s.loadPWs()
		if err != nil {
			return "", err
		}
	}
	return s.cr.OS, nil
}

//Returns BIOS Password.
func (s *ostash) ReadBiosPass() (string, error) {
	if len(s.cr.OS) == 0 {
		err := s.loadPWs()
		if err != nil {
			return "", err
		}
	}
	return s.cr.BIOS, nil
}

//Returns IPMI Password.
func (s *ostash) ReadIPMIPass() (string, error) {
	if len(s.cr.OS) == 0 {
		err := s.loadPWs()
		if err != nil {
			return "", err
		}
	}
	return s.cr.IPMI, nil
}

// Asks user to input shell password. Compares to stored pw. Reboots if no
// match - ONLY returns if password matches.
func (s *ostash) RequestShellPassword() {
	//must be fatal - otherwise grants access with no pw check
	log.Fatalf("ostash.RequestShellPassword: unimplemented")
}
