// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package mdata

import (
	"encoding/json"
	"gprovision/pkg/appliance"
	"gprovision/pkg/common"
	"gprovision/pkg/common/fr"
	"gprovision/pkg/common/stash"
	"gprovision/pkg/common/strs"
	"gprovision/pkg/hw/uefi"
	"gprovision/pkg/log"
	steps "gprovision/pkg/mfg/configStep"
	"gprovision/pkg/mfg/qa"
	"gprovision/pkg/net/xfer"
	"gprovision/pkg/recovery/disk"
	"os"
	fp "path/filepath"
	"strings"
)

//FIXME template every url, allowing to use the proto/IP/prefix the json came from
type MfgDataStruct struct {
	ApplianceJsonUrl   string         `json:",omitempty"`
	Files              []*xfer.TVFile // .Dest is relative to root of recovery volume, i.e. Image/pkg.name.version.upd
	LogEndpoint        string
	CredentialEndpoint string
	StashFiles         []*xfer.TVFile //list of files for use in Stasher impl. implementation-defined.
	ValidationData     []qa.Specs
	CustomPlatCfgSteps steps.PlatformConfigs
}

func Parse(url string) (mds *MfgDataStruct) {
	if !strings.HasSuffix(url, ".json") {
		log.Log(url)
		log.Fatalf("mfg data must be .json")
	}
	data, err := xfer.GetFile(url)
	if err != nil {
		log.Logln(err)
		log.Fatalf("error retrieving mfg data file")
	}
	mds = &MfgDataStruct{}
	err = json.Unmarshal(data, mds)
	if err != nil {
		log.Logln(err)
		log.Fatalf("error unmarshalling mfg data")
	}
	if mds.ApplianceJsonUrl != "" {
		appliance.LoadJson(mds.ApplianceJsonUrl)
	}

	stash.SetData(mds)
	return
}

// Copy image to RECOVERY volume, along with kernel, boot menu, anything else
// listed in mfgDataStruct.
func (m *MfgDataStruct) WriteFiles(r *disk.Filesystem) {
	var err error
	for _, f := range m.Files {
		//extern retry lib: https://medium.com/@matryer/retrying-in-golang-quicktip-f688d00e650a#.n7cyeywgm
		if f.Dest == "" {
			base := f.Basename()
			if isImage(base) {
				f.Dest = fp.Join(r.Path(), "Image", base)
			} else if base == strs.BootKernel() && uefi.BootedUEFI() {
				f.Dest = fp.Join(r.Path(), "ESP", base)
				err = os.Symlink(fp.Join("ESP", base), fp.Join(r.Path(), base))
				if err != nil {
					log.Fatalf("creating ESP symlink for %s: %s", base, err)
				}
			} else {
				f.Dest = fp.Join(r.Path(), base)
			}
			log.Logf("%s lacks Dest, assuming %s", f.Src, f.Dest)
		} else if f.Dest[0] != byte('/') {
			f.Dest = fp.Join(r.Path(), f.Dest)
		}
		checkDest(f.Dest)
		f.UseIntermediateDir("/tmp/")
		err = f.GetWithRetry()
		if err == nil {
			err = f.Finalize()
		}
		if err != nil {
			log.Fatalf("error while writing %s: %s", f.Dest, err)
		}
	}
	if !uefi.BootedUEFI() {
		r.WriteFallbackBootMenu()
	}
}

func isImage(name string) bool {
	return strings.HasPrefix(name, strs.ImgPrefix()) && strings.HasSuffix(name, ".upd")
}

//checks a path to ensure it's a sane destination path
func checkDest(d string) {
	for {
		//remove leading slashes
		if len(d) == 0 || d[0] != '/' {
			break
		}
		d = d[1:]
	}
	pfx := "Image/"
	if strings.HasPrefix(d, pfx) {
		return
	}
	if strings.HasPrefix(strings.ToLower(d), strings.ToLower(pfx)) {
		log.Fatalf("Image/ dir MUST be capitalized: %s", d)
	}
}

// Returns qa.Specs matching given codename
func (m *MfgDataStruct) FindSpecs(codeName string) (s *qa.Specs) {
	for _, v := range m.ValidationData {
		if v.DevCodeName == codeName {
			s = &v
			return
		}
	}
	log.Fatalf("%s not in validation data", codeName)
	return
}

// Go through list of files, return the first for which isImage() returns true.
func (m *MfgDataStruct) FindImage() *xfer.TVFile {
	for _, f := range m.Files {
		if isImage(f.Basename()) {
			return f
		}
	}
	return nil
}

// Writes FR Config file.
func (m *MfgDataStruct) FRConfig(recov common.Pather, noDelete bool, bootArgs string) {
	fr.SetXLog(m.LogEndpoint)
	fr.SetPreserve(noDelete)
	fr.SetBootArgs(bootArgs)
	err := fr.Persist()
	if err != nil {
		log.Logln(err)
		log.Fatalf("failed to write FR json")
	}
}

//implements StashData
var _ common.StashData = (*MfgDataStruct)(nil)

//StashData.CredEP()
func (m *MfgDataStruct) CredEP() string {
	return m.CredentialEndpoint
}

//StashData.StashFileList()
func (m *MfgDataStruct) StashFileList() (tvf []common.TransferableVerifiableFile) {
	for _, s := range m.StashFiles {
		tvf = append(tvf, s)
	}
	return
}
