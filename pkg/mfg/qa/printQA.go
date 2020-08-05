// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package qa

import (
	"bytes"
	"html/template"
	"strings"

	"github.com/purecloudlabs/gprovision/pkg/appliance"
	"github.com/purecloudlabs/gprovision/pkg/common/rkeep"
	"github.com/purecloudlabs/gprovision/pkg/log"
	steps "github.com/purecloudlabs/gprovision/pkg/mfg/configStep"
	"github.com/purecloudlabs/gprovision/pkg/mfg/mfgflags"
	"github.com/purecloudlabs/gprovision/pkg/net/xfer"
)

//used to fill in html template
type qavData struct {
	Pass              bool
	SN                string
	Model             string
	Cpus              CPUInfo
	NicEepromFlash    bool
	NumPci, NumUsb    int
	Img               *xfer.TVFile
	ImageCksumMatches bool
	mfgflags          bool
	CfgSteps          []string
}

func (d qavData) hardcopy() (buf bytes.Buffer) {
	if d.mfgflags || appliance.IdentifiedViaFallback() {
		if d.mfgflags {
			log.Msg("Mfg flags altered behavior. Not printing QA report.")
		} else {
			log.Msg("Platform identification overridden. Not printing QA report.")
			log.Log("Identification overridden via file or env var.")
		}
		log.Logf("QA report would be based upon the following:\n%#v", d)
		return
	}
	if !d.Pass {
		log.Msg("Did not pass QA - not printing report")
		return
	}
	d.Img.Dest = strings.TrimSuffix(d.Img.Basename(), ".upd")
	err := qaTmpl.Execute(&buf, d)
	if err != nil {
		log.Fatalf("error producing qa data: %s", err)
		buf.Truncate(0)
	}
	return
}
func (d qavData) Hardcopy() {
	buf := d.hardcopy()
	if buf.Len() == 0 {
		return
	}
	log.Msg("Sending QA document for printing...")
	rkeep.StoreDocument(d.SN+"_qav.htm", rkeep.PrintedDocQAV, buf.Bytes())
}

func QASummary(img *xfer.TVFile, detected *Specs, plat *appliance.Variant, cfgsteps steps.ConfigSteps) (d qavData) {
	if mfgflags.BehaviorAltered {
		d.mfgflags = true
	}
	d.Model = plat.PrettyName()
	d.SN = plat.SerNum()
	d.Model = detected.DevCodeName
	d.Cpus = detected.CPUInfo
	d.NicEepromFlash = (detected.NumOUINics != 0)
	d.NumPci = len(detected.Devices.PCI)
	d.NumUsb = len(detected.Devices.USB)

	// TODO firmware versions?
	d.Img = img

	if mfgflags.Flag(mfgflags.NoWrite) {
		log.Logf("image not written - cannot verify")
	} else {
		log.Msg("sync image, verify...")
		err := d.Img.Verify()
		if err == nil {
			d.ImageCksumMatches = true
			log.Msg("image verified")
		} else {
			log.Logf("verifying img: %s", err)
			log.Fatalf("After writing image to recovery, checksum does not match")
		}
	}
	if d.ImageCksumMatches {
		//check other things?
		d.Pass = true
	}
	for _, s := range cfgsteps {
		d.CfgSteps = append(d.CfgSteps, s.Name)
	}
	return
}

// cause bindata.go to be generated from files in the given dir
//go:generate ../../../bin/go-bindata -prefix=../../../proprietary/data/qa -pkg=$GOPACKAGE ../../../proprietary/data/qa

var qaTmpl *template.Template

func init() {
	//template that may be embedded by go-bindata
	q, err := Asset("qa.tmpl.html")
	if err != nil {
		q = []byte(`example qa template (see source):
{{.SN}} {{ .Img.Sha1 }}
{{.Model}} {{ .Cpus.Cores }}
{{ .NumPci }} {{ .NumUsb }}
{{ range .CfgSteps -}}{{ . }}{{ end -}}
`)
	}
	qaTmpl = template.Must(template.New("qa").Parse(string(q)))
}
