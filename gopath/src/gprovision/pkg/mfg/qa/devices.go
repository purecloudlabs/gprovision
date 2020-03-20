// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package qa

import (
	"bufio"
	"encoding/json"
	"fmt"
	"gprovision/pkg/log"
	"gprovision/pkg/mfg/mfgflags"
	"io/ioutil"
	"os"
	fp "path/filepath"
	"strconv"
	"strings"
)

type Hexadecimal uint64

func (h *Hexadecimal) UnmarshalJSON(b []byte) (err error) {
	var s string
	err = json.Unmarshal(b, &s)
	if err == nil {
		var v uint64
		v, err = strconv.ParseUint(s, 0, 64) //base==0 is necessary for ParseUint to accept 0x prefix
		*h = Hexadecimal(v)
	}
	if err != nil {
		log.Logf("unmarshalling %s: %s", string(b), err)
	}
	return
}
func (h Hexadecimal) MarshalJSON() ([]byte, error) {
	return json.Marshal(fmt.Sprintf("0x%04x", h))
}

type BusDevice struct {
	HumanDescription string      `json:",omitempty"`
	Vendor           Hexadecimal `json:",omitempty"`
	Device           Hexadecimal `json:",omitempty"`
	Class            Hexadecimal `json:",omitempty"`
	Quantity         uint64      `json:",omitempty"`
	dev              string
}

func (bd BusDevice) String() string {
	return fmt.Sprintf("'%s' (vendor 0x%04x device 0x%04x class 0x%04x)", bd.HumanDescription, bd.Vendor, bd.Device, bd.Class)
}

const (
	pcidevs = "/sys/bus/pci/devices"
	usbdevs = "/sys/bus/usb/devices"
)

type BusDeviceList []*BusDevice
type PciDevices BusDeviceList

func (p *PciDevices) Populate() {
	err := bdpopulate((*BusDeviceList)(p), pcidevs)
	if err != nil {
		log.Logf("populating pci devices: %s", err)
	}
}
func (required PciDevices) Compare(detected PciDevices) (errors int) {
	errors = bdcompare(BusDeviceList(required), BusDeviceList(detected))
	if errors == 0 {
		log.Msg("+++ PCI Devices: match +++")
	} else {
		log.Msgf("!!! PCI Devices: %d errors !!!", errors)
	}
	return
}
func (b *BusDevice) SetDescription() {
	c, s, _, m, _ := b.ReadUdevStrings()
	if m != "" {
		b.HumanDescription = m
	} else if s != "" {
		b.HumanDescription = s
	} else if c != "" {
		b.HumanDescription = c
	} else {
		if strings.HasPrefix(b.dev, "/sys/bus/usb/") {
			m, _ := ioutil.ReadFile(fp.Join(b.dev, "manufacturer"))
			p, _ := ioutil.ReadFile(fp.Join(b.dev, "product"))
			if len(m) > 0 {
				b.HumanDescription = strings.TrimSpace(string(m)) + " "
			}
			if len(p) > 0 {
				b.HumanDescription += strings.TrimSpace(string(p))
			}
			if len(b.HumanDescription) == 0 {
				b.HumanDescription = "unknown usb device"
			}
		} else {
			b.ClassCode()
		}
	}
}

//unfortunately, seems this won't work for the mfg app - because we lack the device database??
func (b *BusDevice) ReadUdevStrings() (class, subclass, vendor, model string, err error) {
	dtype := "pci"
	if strings.Contains(b.dev, "usb") {
		dtype = "usb"
	}
	f, err := os.Open("/run/udev/data/+" + dtype + ":" + fp.Base(b.dev))
	if err != nil {
		return
		//log.Logf("err %s reading udev data for %s", err, b)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		l := scanner.Text()
		s := strings.SplitN(strings.TrimSpace(l), "=", 2)
		if len(s) != 2 {
			continue
		}
		switch s[0] {
		case "E:ID_PCI_CLASS_FROM_DATABASE":
			//e.g. Network controller
			class = s[1]
		case "E:ID_USB_CLASS_FROM_DATABASE":
			//e.g. Hub
			class = s[1]
		case "E:ID_PCI_SUBCLASS_FROM_DATABASE":
			//e.g. Ethernet controller
			subclass = s[1]
		case "E:ID_VENDOR_FROM_DATABASE":
			vendor = s[1]
		case "E:ID_MODEL_FROM_DATABASE":
			model = s[1]
		default:
		}
	}
	return
}

var PCIClassCodes map[Hexadecimal]string

func init() {
	PCIClassCodes = make(map[Hexadecimal]string)
	PCIClassCodes[0x0100] = "SCSI controller"
	PCIClassCodes[0x0104] = "RAID controller"
	PCIClassCodes[0x0106] = "SATA controller"
	PCIClassCodes[0x0107] = "SAS controller"
	PCIClassCodes[0x0200] = "Ethernet Controller"
	PCIClassCodes[0x0300] = "VGA Controller"
	PCIClassCodes[0x0403] = "Audio Device"
	PCIClassCodes[0x0600] = "Host Bridge"
	PCIClassCodes[0x0601] = "ISA Bridge"
	PCIClassCodes[0x0604] = "PCI Bridge"
	PCIClassCodes[0x0780] = "Communication controller"
	PCIClassCodes[0x0800] = "PIC"
	PCIClassCodes[0x0880] = "System peripheral"
	PCIClassCodes[0x0c03] = "USB Controller"
	PCIClassCodes[0x0c05] = "SMBus"
	PCIClassCodes[0x1101] = "Performance counters"
	PCIClassCodes[0x1180] = "Signal processing controller"
	PCIClassCodes[0xff00] = "Unassigned class"
}

//set human-readable description for common pci device classes
func (b *BusDevice) ClassCode() {
	desc, ok := PCIClassCodes[b.Class]
	if ok {
		b.HumanDescription = desc
	} else {
		b.HumanDescription = "unknown device class"
	}
}

type UsbDevices BusDeviceList

func (u *UsbDevices) Populate() {
	err := bdpopulate((*BusDeviceList)(u), usbdevs)
	if err != nil {
		log.Logf("populating usb devices: %s", err)
	}
}

func (required UsbDevices) Compare(detected UsbDevices) (errors int) {
	errors = bdcompare(BusDeviceList(required), BusDeviceList(detected))
	if errors == 0 {
		log.Msg("+++ USB Devices: match +++")
	} else {
		log.Msgf("!!! USB Devices: %d errors !!!", errors)
	}
	return
}

//return paths to devices
func listDevs(t string) (devs []string, err error) {
	entries, err := ioutil.ReadDir(t)
	if err != nil {
		err = fmt.Errorf("error %s reading dir %s", err, t)
		return
	}
	for _, e := range entries {
		devs = append(devs, fp.Join(t, e.Name()))
	}
	return
}

var errSkipThisDevice = fmt.Errorf("skip this device")

//read device data for a given device, returning vendor/device/class
func readDev(path string) (vendor, device, class Hexadecimal, err error) {
	if strings.HasPrefix(path, "/sys/bus/usb/") {
		//USB
		vendor, err = fatoi(path + "/idVendor")
		if os.IsNotExist(err) {
			return 0, 0, 0, errSkipThisDevice
		}
		if err == nil {
			device, err = fatoi(path + "/idProduct")
		}
		if err == nil {
			class, err = fatoi(path + "/bDeviceClass")
		}
		return
	}
	//PCI
	vendor, err = fatoi(path + "/vendor")
	if err == nil {
		device, err = fatoi(path + "/device")
	}
	if err == nil {
		var c Hexadecimal
		c, err = fatoi(path + "/class")
		//low byte is apparently Prog-IF, which we don't care about
		class = c >> 8
	}
	return
}

//atoi on a file. input is always treated as hex, with or without 0x prefix
func fatoi(file string) (h Hexadecimal, err error) {
	f, err := ioutil.ReadFile(file)
	if err == nil {
		in := strings.TrimSpace(string(f))
		var v uint64
		if strings.HasPrefix(in, "0x") {
			v, err = strconv.ParseUint(in, 0, 64)
		} else {
			v, err = strconv.ParseUint(in, 16, 64)
		}
		h = Hexadecimal(v)
	}
	return
}

//reads device data from given dir in /sys, populating a list of bus devices
func bdpopulate(l *BusDeviceList, source string) (err error) {
	devs, err := listDevs(source)
	if err != nil {
		return
	}
	for _, dev := range devs {
		added := false
		vendor, device, class, err := readDev(dev)
		if mfgflags.Verbose {
			log.Logf("%s v=0x%x d=0x%x c=0x%x", dev, vendor, device, class)
		}
		if err == errSkipThisDevice {
			if mfgflags.Verbose {
				log.Logf("skipping device %s", dev)
			}
			continue
		}
		if err != nil {
			log.Logf("error reading data for device %s: %s", dev, err)
			continue
		}
		for _, d := range *l {
			if d.Vendor == vendor && d.Device == device && d.Class == class {
				if mfgflags.Verbose {
					log.Logf("multiple %s", d.HumanDescription)
				}
				d.Quantity += 1
				added = true
				break
			}
		}
		if !added {
			if mfgflags.Verbose {
				log.Logf("add one 0x%x", device)
			}
			var d BusDevice
			d.Vendor = vendor
			d.Device = device
			d.Class = class
			d.Quantity = 1
			d.dev = dev
			d.SetDescription()
			*l = append(*l, &d)
		}
	}
	return
}

//compares list of required devices with detected devices
//reports error if required device isn't among those detected
//detected list will usually be much longer than the required list
func bdcompare(required BusDeviceList, detected BusDeviceList) (errors int) {
	for _, r := range required {
		found := false
		for _, d := range detected {
			if d.Vendor == r.Vendor && d.Device == r.Device && d.Class == r.Class {
				if d.Quantity != r.Quantity {
					log.Logf("wrong quantity of %s: want %d, got %d", r, r.Quantity, d.Quantity)
					errors += 1
				}
				found = true
				if mfgflags.Verbose {
					log.Logf("%s matches %s", r, d)
				}
				break
			} else if mfgflags.Verbose {
				log.Logf("%s doesn't match %s", r, d)
			}
		}
		if !found && r.Quantity != 0 {
			log.Logf("did not find any %s", r)
			errors += 1
		}
	}
	if errors != 0 {
		list := "devices that were detected:\n"
		for _, d := range detected {
			list += fmt.Sprintf("%s, qty %d\n", d, d.Quantity)
		}
		log.Log(list)
	}
	return
}
