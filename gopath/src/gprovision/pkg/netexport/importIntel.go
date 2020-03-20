// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package netexport

import (
	"encoding/csv"
	"fmt"
	futil "gprovision/pkg/fileutil"
	"gprovision/pkg/log"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	fp "path/filepath"
	"strconv"
	"strings"
)

type IntelMap map[string]*WinNic

func NewIntelMap() IntelMap { return make(map[string]*WinNic) }

func GetIntelData() (IntelMap, error) {
	dir, err := ioutil.TempDir("", "netexport")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	sr := exec.Command(powershellExe, "-file", `C:\Program Files\Intel\DMIX\SaveRestore.ps1`, "-Action", "save")
	sr.Dir = dir
	var out []byte
	out, err = sr.CombinedOutput()
	if err == nil && !strings.Contains(string(out), "Performing a save\n") {
		log.Logln("powershell: unexpected output", string(out))
		//try to continue - maybe the file exists anyway
	}
	if err != nil {
		log.Logln("cmd: ", sr.Args, " error: ", err, " output: ", string(out))
		return nil, err
	}
	return parseOutput(dir)
}

//parse SaveRestore.ps1 output from dir
//only read Saved_Config.txt, as data in Saved_StaticIP.txt is ipv4 only - must get ip info elsewhere
func parseOutput(dir string) (intelData IntelMap, err error) {
	cfgf := fp.Join(dir, "Saved_Config.txt")
	intelData = NewIntelMap()

	var cfg io.ReadSeeker
	cfg, err = os.Open(cfgf)
	if err != nil {
		return
	}
	err = intelData.loadIntel(cfg)
	return
}

//parse a file written by SaveRestore.ps1
func (ifaces IntelMap) loadIntel(data io.ReadSeeker) (err error) {
	currentSection := ""
	//currently, we discard type data from BOM. csv reader seems to work without the data. should we use it? if so, how?
	//strangely, the files have a UTF8 BOM (`hexdump`, `file`) but the bytes, if printed here, are FF FE - indicating UTF16LE
	_, err = futil.DetectBOM(data)
	if err != nil {
		return
	}
	r := csv.NewReader(data)
	r.FieldsPerRecord = -1 //don't check number of fields per record
	for {
		var record []string
		record, err = r.Read()
		if err == io.EOF {
			err = nil
			break
		}
		if err != nil {
			log.Logf("error %s reading a record\n", err)
			return
		}
		if len(record) == 0 {
			continue
		}
		if len(record) == 1 {
			if strings.HasSuffix(record[0], "Start") {
				if currentSection != "" {
					return fmt.Errorf("parse error, found %v inside %s\n", record, currentSection)
				}
				currentSection = strings.TrimSuffix(record[0], "Start")
				// fmt.Fprintf(log,"section %s\n", currentSection)
			} else if strings.HasSuffix(record[0], "End") {
				if currentSection == "" {
					return fmt.Errorf("parse error, found %v while not in a section\n", record)
				}
				if currentSection != strings.TrimSuffix(record[0], "End") {
					return fmt.Errorf("parse error, found end %v in %q\n", record, currentSection)
				}
				currentSection = ""
			} else {
				return fmt.Errorf("parse error, found unexpected '%v'\n", record)
			}
			continue
		}
		switch currentSection {
		case "Adapters":
			//"Intel(R) Ethernet Server Adapter I350-T4 #3","Intel(R) Ethernet Server Adapter I350-T4 #3","0026FDA00D56","VEN_8086&DEV_1521&SUBSYS_80860001&REV_01&3&0","3:0:0:0"
			//"WindowsName","WindowsName","MAC","Win PCI ident","PCI Slot"
			if len(record) != 5 {
				return fmt.Errorf("parse error, expect lines in Adapters section to have 5 sections: '%v'\n", record)
			}
			winName := record[0]
			_, ok := ifaces[winName]
			if ok {
				return fmt.Errorf("interface with name '%s' already exists", winName)
			}
			addr, err := convertMac(record[2])
			if err != nil {
				log.Logf("error, bad mac %s: %s\n", record[2], err)
				continue
			}
			ifaces[winName] = &WinNic{
				WinName: winName,
				Mac:     StringyMac{addr},
			}
		case "AdapterSettings":
			//"WindowsName","Option","Arg1","Arg2","MAC","Win PCI ident","PCI Slot"
			if len(record) != 7 {
				return fmt.Errorf("parse error, expect lines in AdapterSettings section to have 7 sections: '%v'\n", record)
			}

			winName := record[0]

			iface, ok := ifaces[winName]
			if ok {
				option := record[1]
				switch option {
				case "ConnectionName":
					if strings.TrimSpace(record[2]) == "" {
						panic("interface with unknown name")
					}
					iface.FriendlyName = record[2]
				case "EnableDHCP":
					//ignore - intel only gives us ipv4 data
				case "AdaptiveIFS":
				case "LinkNegotiationProcess":
				case "SipsEnabled":
				case "DMACoalescing":
				case "EEELinkAdvertisement":
				case "EnableLLI":
				case "EnablePME":
				case "ITR":
				case "LLIPorts":
				case "LogLinkStateEvent":
				case "MasterSlave":
				case "NetworkAddress":
				case "ReduceSpeedOnPowerDown":
				case "WaitAutoNegComplete":
				case "WakeOnLink":
				default:
					if len(option) > 0 && option[0] != '*' {
						log.Logf("ignoring adapter setting %v for '%s'\n", option, winName)
					}
				}
			} else {
				return fmt.Errorf("parse error, interface '%s' not in map: %v\n", winName, record)
			}
		case "Teams":
			fallthrough
		case "TeamSettings":
			log.Logf("teaming is unsupported, ignoring [%s] %v\n", currentSection, record)
		case "Vlans":
			//"Intel(R) Ethernet Server Adapter I350-T4 #4","88","VLAN88"
			parentName := record[0]
			parent, ok := ifaces[parentName]
			if !ok {
				return fmt.Errorf("missing parent for %v", record)
			}
			parent.HasVLANChildren = true
			winName := parentName + " - VLAN : " + record[2]
			_, ok = ifaces[winName]
			if ok {
				return fmt.Errorf("vlan %s already exists", winName)
			}
			var vlan uint64
			vlan, err = strconv.ParseUint(record[1], 10, 64)
			if err != nil {
				log.Logf("parse vlan: %s", err)
			}
			ifaces[winName] = &WinNic{
				WinName: winName,
				IsVLAN:  true,
				Mac:     parent.Mac,
				VLAN:    vlan,
			}
		case "VlanSettings":
			//"Intel(R) Ethernet Server Adapter I350-T4 #4","Intel(R) Ethernet Server Adapter I350-T4 #4 - VLAN : VLAN88","88","ConnectionName","Ethernet 2"
			//Parent,Name,VLAN, FriendlyName
			winName := record[1]
			iface, ok := ifaces[winName]
			if !ok {
				return fmt.Errorf("vlan %s does not exist", winName)
			}
			option := record[3]
			switch option {
			case "ConnectionName":
				if strings.TrimSpace(record[4]) == "" {
					panic("vlan with unknown name")
				}
				//Port 3 - VLAN 2
				iface.FriendlyName = record[4]
			case "EnableDHCP":
				//ignore - intel only gives us ipv4 data
			default:
				log.Logf("ignoring vlan option %s for '%s'\n", option, winName)
			}
		case "NICPARTSettings": //partitioning for SR-IOV?
			return fmt.Errorf("%s support is not implemented, ignoring %v\n", currentSection, record)

		default:
			return fmt.Errorf("section '%q', ignoring line '%v'\n", currentSection, record)
		}
	}
	return
}

//lowercase the MAC, insert colons
func convertMac(m string) (net.HardwareAddr, error) {
	nm := ""
	for idx, c := range m {
		if idx != 0 && idx%2 == 0 {
			nm += ":"
		}
		nm += string(c)
	}
	return net.ParseMAC(nm)
}
