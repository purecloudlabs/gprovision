// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package networkd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"gprovision/pkg/appliance"
	"gprovision/pkg/hw/nic"
	inet "gprovision/pkg/net"
	nx "gprovision/pkg/netexport"
	"io"
	"io/ioutil"
	"net"
	"strings"
	"testing"
)

//func (nic WinNic) toNetD() (cfgs []configFile)
func TestExport(t *testing.T) {
	mac, err := net.ParseMAC("00:26:FD:A0:0D:51")
	if err != nil {
		t.Error(err)
	}
	ip, in, _ := net.ParseCIDR("10.155.8.123/24")
	in.IP = ip //because otherwise it's the subnet
	ipn := inet.IPNet{*in}
	ns1 := net.ParseIP("8.8.4.4")
	ns2 := net.ParseIP("1.2.3.4")
	gw := net.ParseIP("10.155.8.1")
	_, dest, _ := net.ParseCIDR("0.0.0.0/0")
	nic := nx.WinNic{
		WinName:      "Intel(R) I350 Gigabit Network Connection",
		FriendlyName: "Port 3",
		Mac:          nx.StringyMac{mac},
		DHCP4:        false,
		DHCP6:        false,
		IPs:          []inet.IPNet{ipn},
		NameServers:  []net.IP{ns1, ns2},
		Routes:       []nx.Route{{inet.IPNet{*dest}, gw, 1234}},
	}

	cfgs := toNetD(&nic, nil)
	if len(cfgs) != 2 {
		t.Errorf("want 2")
	}
	if t.Failed() {
		buf := new(bytes.Buffer)
		printNic(buf, cfgs)
		t.Log(buf.String())
	}
}
func printNic(buf io.Writer, cfgs []configFile) {
	for _, c := range cfgs {
		fmt.Fprintf(buf, "\n=============== %s =============\n%s", c.name, string(c.data))
	}
}

type ifRequirements struct {
	index          int
	numVlans       int
	linkMatchIsMac bool
	hasNetdev      bool
	numRoutes      int
	numIPs         int
	numDNS         int
}

func (cfgs ifConfig) check(t *testing.T, req ifRequirements) (failed bool) {
	var hasNetdev, hasLink, hasNetwork bool
	for _, cfg := range cfgs {
		dot := strings.LastIndex(cfg.name, ".")
		switch cfg.name[dot+1:] {
		case "link":
			hasLink = true
			//linkMatchIsMac
			matchMac := bytes.Contains(cfg.data, []byte("[Match]\nMACAddress="))
			if req.linkMatchIsMac != matchMac {
				failed = true
				t.Errorf("link: incorrect [Match]")
			}
		case "network":
			hasNetwork = true
			//numVlans
			emptyvl := bytes.Count(cfg.data, []byte("\nVLAN=\n"))
			if emptyvl != 0 {
				failed = true
				t.Errorf("network: %d empty vlans (\\nVLAN=\\n)", emptyvl)
			}
			nv := bytes.Count(cfg.data, []byte("\nVLAN="))
			if nv != req.numVlans {
				failed = true
				t.Errorf("network: want %d vlans, got %d", req.numVlans, nv)
			}
			//numRoutes
			nr := bytes.Count(cfg.data, []byte("[Route]"))
			if nr != req.numRoutes {
				failed = true
				t.Errorf("network: want %d routes, got %d", req.numRoutes, nr)
			}
			//numIPs
			ni := bytes.Count(cfg.data, []byte("\nAddress="))
			if ni != req.numIPs {
				failed = true
				t.Errorf("network: want %d Addresses, got %d", req.numIPs, ni)
			}
			//numDNS
			nd := bytes.Count(cfg.data, []byte("DNS="))
			if nd != req.numDNS {
				failed = true
				t.Errorf("network: want %d DNS, got %d", req.numDNS, nd)
			}
		case "netdev":
			hasNetdev = true
		default:
			failed = true
			t.Errorf("file %s: unknown ext %s", cfg.name, cfg.name[dot:])
			continue
		}
	}
	if hasNetdev != req.hasNetdev {
		failed = true
		t.Errorf("netdev file presence: want=%t, got=%t", req.hasNetdev, hasNetdev)
	}
	if !hasNetwork {
		failed = true
		t.Errorf("missing network file")
	}
	if !hasLink {
		failed = true
		t.Errorf("missing link file")
	}
	return
}

//capitalize (or call) to print loaded config (for comparison with input file)
func testPrintComplexCfg(t *testing.T) {
	ifmap := importJson("data/complex.json")
	out, err := json.MarshalIndent(ifmap, "", "    ")
	if err != nil {
		t.Errorf("%s", err)
	}
	t.Log(string(out))
}

//set up complex config (loaded from json) and check that the output files match
func TestComplexExport(t *testing.T) {
	requirements := []ifRequirements{
		{17, 0, true, false, 0, 0, 0},
		{9990, 2, true, false, 0, 0, 0},
		{23, 0, false, true, 2, 0, 0},
		{27, 0, false, true, 0, 0, 0},
		{13, 0, true, false, 1, 1, 2},
		{14, 0, true, false, 1, 3, 2},
		{9991, 1, true, false, 0, 0, 0},
		{26, 0, false, true, 1, 4, 4},
		{12, 0, true, false, 0, 0, 0},
		{16, 0, true, false, 0, 0, 0},
	}
	ifmap := importJson("data/complex.json")
	noErrs := new(bytes.Buffer)
	for i, req := range requirements {
		nic := ifmap[req.index]
		vlans := ifmap.VlanChildren(nic)
		cfgs := toNetD(nic, vlans)
		errs := cfgs.check(t, req)
		if errs {
			errBuf := new(bytes.Buffer)
			t.Errorf("error(s) at line %d", i)
			printNic(errBuf, cfgs)
			t.Log(errBuf)
		} else {
			printNic(noErrs, cfgs)
		}
	}
	if t.Failed() {
		t.Logf("interface configs with no errors:\n============================================================\n%s\n", noErrs.Bytes())
	}
}
func importJson(j string) (ifmap nx.IfMap) {
	ifmap = nx.NewIfMap()
	data, err := ioutil.ReadFile(j)
	if err != nil {
		panic(err)
	}
	err = json.Unmarshal(data, &ifmap)
	if err != nil {
		panic(err)
	}
	return
}

//func defaults(sortedNics []nic.Nic,platform *appliance.Variant) (ifaces nx.IfMap)
func TestDefaults(t *testing.T) {
	macs := []string{"00:1b:21:c6:d0:10", "d4:3d:7e:2d:25:05", "ba:fa:1f:78:e6:3d", "d4:3d:7e:77:88:99"}
	ni := appliance.NICInfo{
		SharedDiagPorts:    []int{0},
		DefaultNamesNoDiag: []string{"p0", "p1"},
		MACPrefix:          []string{"00:1b:21", "d4:3d:7e"},
	}
	plat := appliance.TestSetup(appliance.Variant_{NICInfo: ni}, "", "", "", "")
	var nlist nic.NicList
	for i, m := range macs {
		n, err := nic.TestNic(fmt.Sprintf("e%d", i), m, nil)
		if err != nil {
			t.Error(err)
		}
		nlist = append(nlist, n)
	}
	nics := nlist.Sort().FilterMACs(plat.MACPrefixes())

	ifaces := defaults(nics, plat)
	if len(ifaces) != 3 {
		t.Errorf("wrong number")
	}

	order := []struct {
		name   string
		macIdx int
	}{{"DIAG", 0}, {"p0", 1}, {"p1", 3}}
	for i, v := range order {
		if ifaces[i].FriendlyName != v.name {
			t.Errorf("%d: want name %s, got %s", i, v.name, ifaces[i].FriendlyName)
		}
		if ifaces[i].Mac.String() != macs[v.macIdx] {
			t.Errorf("%d: want mac %s, got %s", i, macs[v.macIdx], ifaces[i].Mac)
		}
	}
	if t.Failed() {
		for i, f := range ifaces {
			t.Logf("%d: %s - %s", i, f.FriendlyName, f.Mac)
		}
	}
	dhcpYesCount := 0
	dhcpCount := 0
	for _, nic := range ifaces {
		vlans := ifaces.VlanChildren(nic)
		cfgs := toNetD(nic, vlans)
		for _, c := range cfgs {
			if strings.Contains(string(c.data), "DHCP=yes") {
				dhcpYesCount++
			}
			if strings.Contains(string(c.data), "DHCP") {
				dhcpCount++
			}
			t.Logf("\n==============\n%s: %s\n%s", nic.FriendlyName, c.name, string(c.data))
		}
	}
	if dhcpYesCount != 2 {
		t.Errorf("expect 2 of 'DHCP=yes', got %d", dhcpYesCount)
	}
	if dhcpCount != 3 {
		//diag port's file says "# no DHCP"
		t.Errorf("expect 3 of 'DHCP', got %d", dhcpCount)
	}
}
