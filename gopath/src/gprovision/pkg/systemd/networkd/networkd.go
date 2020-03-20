// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

//Package networkd can be used to write config files for systemd-networkd. Used
//in conjunction with gprovision/pkg/netexport.
package networkd

import (
	"archive/tar"
	"bytes"
	"fmt"
	"gprovision/pkg/log"
	nx "gprovision/pkg/netexport"
	"io/ioutil"
	"os"
	fp "path/filepath"
	"strings"
	"text/template"
	"time"
)

// Export exports config to a tarball.
func Export(ifaces nx.IfMap, tarball string) (err error) {
	tb, err := os.Create(tarball)
	if err != nil {
		return
	}
	defer tb.Close()
	tball := tar.NewWriter(tb)
	defer tball.Close()
	now := time.Now()
	for _, nic := range ifaces {
		vlans := ifaces.VlanChildren(nic)
		cfgs := toNetD(nic, vlans)
		for _, c := range cfgs {
			hdr := new(tar.Header)
			hdr.Name = c.name
			hdr.Size = int64(len(c.data))
			hdr.Mode = 0644
			hdr.ModTime = now
			err = tball.WriteHeader(hdr)
			if err != nil {
				return
			}
			_, err = tball.Write(c.data)
			if err != nil {
				return
			}
		}
	}
	return
}

// Write writes config to a set of files.
func Write(ifaces nx.IfMap, dir string) {
	for _, nic := range ifaces {
		vlans := ifaces.VlanChildren(nic)
		cfgs := toNetD(nic, vlans)
		for _, c := range cfgs {
			name := fp.Join(dir, c.name)
			err := ioutil.WriteFile(name, c.data, 0666)
			if err != nil {
				log.Logf("failed to write network config file %s: %s", name, err)
			}
		}
	}
}

type configFile struct {
	name string
	data []byte
}

type ifConfig []configFile

//return up to 3 configFile's
func toNetD(nic *nx.WinNic, vlans []uint64) (cfgs ifConfig) {
	link := linkFile(nic)
	if link.name != "" {
		cfgs = append(cfgs, link)
	}
	network := networkFile(nic, vlans)
	if network.name != "" {
		cfgs = append(cfgs, network)
	}
	if nic.IsVLAN {
		netdev := netdevFile(nic)
		if netdev.name != "" {
			cfgs = append(cfgs, netdev)
		}
	}
	return
}

//Generate safe name for the config file, using MAC and (if present) the VLAN.
func (c *configFile) Name(nic *nx.WinNic, ext string) {
	base := strings.ToLower(strings.Replace(nic.Mac.String(), ":", "", -1))
	if nic.IsVLAN {
		base += fmt.Sprintf("-%d", nic.VLAN)
	}
	c.name = base + "." + ext
}

/* templates
*
* dashes ( `{{-` or `-}}` ) affect whitespace and should be changed with care
 */

var linkTmpl, netTmpl, netdevTmpl *template.Template

func init() {
	funcMap := template.FuncMap{
		"ToUpper": strings.ToUpper,
	}
	linkTmpl = template.Must(template.New("link").Funcs(funcMap).Parse(linkTxt))
	netTmpl = template.Must(template.New("network").Funcs(funcMap).Parse(netTxt))
	netdevTmpl = template.Must(template.New("netdev").Funcs(funcMap).Parse(netdevTxt))
}

const linkTxt = `# {{ .FriendlyName }}
# {{ .WinName }}
[Match]
{{ if .IsVLAN -}}
OriginalName={{ .VlanIfName .VLAN }}
{{- else -}}
MACAddress={{ .Mac.String | ToUpper }}
{{- end }}

[Link]
{{ if .IsVLAN -}}
MACAddress={{ .Mac.String | ToUpper }}
{{ end -}}
Alias={{ .FriendlyName }}
`

func linkFile(nic *nx.WinNic) (link configFile) {
	link.Name(nic, "link")
	out := new(bytes.Buffer)
	err := linkTmpl.Execute(out, nic)
	if err != nil {
		log.Logf("%s: %s", link.name, err)
	}
	link.data = out.Bytes()
	return
}

//for the network file and template, the WinNic struct is embedded in another struct containing additional vlan info
type NetworkFileStruct struct {
	nx.WinNic
	Vlans []uint64
}

const netTxt = `# {{ .FriendlyName }}
# {{ .WinName }}
[Match]
{{ if .IsVLAN -}}
Name={{ .VlanIfName .VLAN }}
{{- else -}}
MACAddress={{ .Mac.String | ToUpper }}
{{- end }}

[Network]
{{- if and .DHCP4 .DHCP6 }}
DHCP=yes
{{- else if .DHCP4 }}
DHCP=ipv4
{{- else if .DHCP6 }}
DHCP=ipv6
{{- else }}
# no DHCP
{{- end }}
LLMNR=no
{{- range .IPs }}
Address={{ . }}
{{- else }}
# no static IP
{{- end }}
{{- range .NameServers }}
DNS={{ . }}
{{- else }}
# no static DNS
{{- end }}
{{- $nic := .}}
{{- range .Vlans }}
VLAN={{ $nic.VlanIfName . }}
{{- end}}

{{- range .Routes }}

[Route]
Gateway={{ .Gateway }}
Metric={{ .Metric }}
Destination={{ .Destination }}
{{- else }}
# no static routes
{{- end }}
`

func networkFile(nic *nx.WinNic, vlans []uint64) (nw configFile) {
	nw.Name(nic, "network")
	out := new(bytes.Buffer)
	nfs := NetworkFileStruct{
		WinNic: *nic,
		Vlans:  vlans,
	}
	err := netTmpl.Execute(out, nfs)
	if err != nil {
		log.Logf("%s: %s", nw.name, err)
	}
	nw.data = out.Bytes()

	return
}

const netdevTxt = `# {{ .FriendlyName }}
# {{ .WinName }}
[NetDev]
Name={{ .VlanIfName .VLAN }}
Kind=vlan

[VLAN]
Id={{ .VLAN }}
`

func netdevFile(nic *nx.WinNic) (netdev configFile) {
	netdev.Name(nic, "netdev")
	out := new(bytes.Buffer)
	err := netdevTmpl.Execute(out, nic)
	if err != nil {
		log.Logf("%s: %s", netdev.name, err)
	}
	netdev.data = out.Bytes()
	return
}
