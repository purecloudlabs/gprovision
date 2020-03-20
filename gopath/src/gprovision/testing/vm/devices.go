// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package vm

import (
	"encoding/base32"
	"fmt"
	"gprovision/pkg/common/strs"
	"gprovision/pkg/log"
	gtst "gprovision/testing"
	"io/ioutil"
	"os"
	fp "path/filepath"
	"strconv"
	"strings"

	"github.com/u-root/u-root/pkg/qemu"
	"github.com/u-root/u-root/pkg/vmtest"
)

//most of these structs will have Cmdline() and KArgs() methods, to satisfy Device.

func Ram5120(o *qemu.Options) error {
	o.Devices = append(o.Devices, qemu.ArbitraryArgs{"-m", "5120"})
	return nil
}

func Ram(i int) qemu.Device {
	return qemu.ArbitraryArgs{"-m", strconv.Itoa(i)}
}

// Adds N adapters, all on a new vm-only network. Only useful if you need
// multiple NICs to show up - not able to communicate with the outside world.
func MultiNet(opts *vmtest.Options, n int) {
	d := qemu.NewNetwork()
	for i := 0; i < n; i++ {
		opts.QEMUOpts.Devices = append(opts.QEMUOpts.Devices, d.NewVM())
	}
}

func AddDevs(devs ...qemu.Device) func(o *qemu.Options) error {
	return func(o *qemu.Options) error {
		o.Devices = append(o.Devices, devs...)
		return nil
	}
}

type Attachment int

const (
	UnknownAttach Attachment = iota
	UsbAttach                //usb2
	UsbXhciAttach            //usb3. shows as usb device (necessary for recovery), but faster.
	IdeAttach                //called 'ide-hd' but looks like sata in vm
	//VirtioAttach //note virtio block device, not virtio-9p
)

type BlockDev struct {
	// attachment method - usb, usb3, ide, ...
	Attachment Attachment

	// size _in bytes_
	Size uint64

	// number of drives, must be > 0
	Count int

	// optional model. known to work with ide.
	Model string

	// filename prefix, to which an (optional) index and extn are added
	// index is only used when Count > 1 and dev # > 1
	// e.g. /abs/path/to/block.qcow, /abs/path/to/block2.qcow, /abs/path/to/block3.qcow, ...
	//
	// if empty, prefix is generated, MustExist must be false.
	//
	// if value is "null", indicates qemu should use the null-copy back end
	// which reports the desired size, but reads as zeros and discards writes.
	FilePfx string

	// if true, file(s) must already exist. if false, will use found files or create new.
	MustExist bool

	// Used for file location if FilePfx is empty; a dir is created if this is empty
	TmpDir string

	// // if true, perform no cleanup
	// NoCleanup bool

	// If non-zero, appended to the device to influence boot order.
	// Implies no '-kernel'. used for recovery.
	BootIndex uint64

	// Arbitrary string that can be used to identify devices in code later on.
	// Not passed to qemu.
	Id string
}

const (
	qcowFmt = "driver=qcow2,node-name=%s,file.driver=file,file.filename=%s"
)

// Derives a unique sequence from the bd pointer's value. Something unique is
// needed to differentiate between devices, and this suffices. Encoded to
// increase the effective bits-per-byte. Must be safe for use as a file name or
// qemu device id, so we encode Base32 rather than Base64 etc.
func (bd *BlockDev) UniqueId() string {
	uniq := base32.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%p", bd)))
	uniq = strings.Replace(uniq, "=", "", -1)
	return uniq[len(uniq)-6:]
}

func (bd *BlockDev) Cmdline() []string {
	if bd.Count == 0 {
		return nil
	}
	var args []string
	var nodes []string //storage node id's for use when defining attachment
	uniq := bd.UniqueId()
	//define the storage nodes
	base := fp.Base(bd.FilePfx)
	if base == "." {
		base = fmt.Sprintf("node%s", uniq)
	}
	for i := 0; i < bd.Count; i++ {
		nodes = append(nodes, base+strconv.Itoa(i))
		if bd.FilePfx == "null" {
			//create null-copy device
			args = append(args,
				"-blockdev",
				fmt.Sprintf("driver=null-co,node-name=%s,size=%d", nodes[i], bd.Size),
			)
			continue
		}
		//if we lack FilePfx and/or TmpDir, create.
		if len(bd.FilePfx) == 0 {
			if len(bd.TmpDir) == 0 {
				var err error
				bd.TmpDir, err = ioutil.TempDir("", "qblk")
				if err != nil {
					log.Fatalf("%s", err)
				}
			}
			bd.FilePfx = fp.Join(bd.TmpDir, "qblk"+uniq)
		}

		file := bd.FilePfx
		if i > 0 {
			file += strconv.Itoa(i + 1)
		}
		file += ".qcow"

		//check if file exists
		if _, err := os.Stat(file); err != nil {
			if bd.MustExist {
				log.Fatalf("%s", err)
			}
			//create missing file
			err = CreateQcow(file, bd.Size)
			if err != nil {
				log.Fatalf("%s", err)
			}
		}
		//create qcow-backed device
		args = append(args,
			"-blockdev",
			fmt.Sprintf(qcowFmt, nodes[i], file))
	}
	//now define how the block device(s) attach to the vm - usb, ide, etc
	switch bd.Attachment {
	case UsbAttach:
		for _, node := range nodes {
			var add string
			if bd.BootIndex > 0 {
				add += fmt.Sprintf(",bootindex=%d", bd.BootIndex)
			}
			if bd.Model != "" {
				add += fmt.Sprintf(",model=%s", bd.Model)
			}
			args = append(args,
				"-device",
				fmt.Sprintf("usb-storage,drive=%s%s", node, add),
			)
		}
	case UsbXhciAttach:
		//more complex - xhci is not included by default, so need to add root port
		args = append(args, "-device", "nec-usb-xhci,id=xhci")
		for _, node := range nodes {
			var add string
			if bd.BootIndex > 0 {
				add += fmt.Sprintf(",bootindex=%d", bd.BootIndex)
			}
			if bd.Model != "" {
				add += fmt.Sprintf(",model=%s", bd.Model)
			}
			args = append(args,
				"-device",
				fmt.Sprintf("usb-storage,bus=xhci.0,drive=%s%s", node, add))
		}
	case IdeAttach:
		for _, node := range nodes {
			var add string
			if bd.BootIndex > 0 {
				add += fmt.Sprintf(",bootindex=%d", bd.BootIndex)
			}
			if bd.Model != "" {
				add += fmt.Sprintf(",model=%s", bd.Model)
			}
			args = append(args,
				"-device",
				fmt.Sprintf("ide-hd,drive=%s%s", node, add))
		}
	default:
		log.Fatalf("unsupported attachment type")
	}
	return args
}

func (bd *BlockDev) KArgs() []string { return nil }

type SmBios struct {
	Raw string
}

func (sb SmBios) Cmdline() []string {
	return []string{"-smbios", sb.Raw}
}

func (sb SmBios) KArgs() []string { return nil }

func SmBios1(uefi bool) *SmBios {
	return &SmBios{
		Raw: "type=1,manufacturer=GPROV_QEMU,product=mfg_test,serial=" + SerNum(uefi),
	}
}

func SmBios1p9(uefi bool) *SmBios {
	return &SmBios{
		Raw: "type=1,manufacturer=GPROV_QEMU,product=9p2k_dev,serial=" + SerNum(false),
	}
}

type OUINic struct {
	mac string
}

func (i OUINic) Cmdline() []string {
	if i.mac == "" {
		i.mac = strs.MacOUI() + ":" + strs.MacOUI()
	}
	return []string{
		"-netdev", "user,id=mynet",
		"-device", "virtio-net-pci,netdev=mynet,mac=" + i.mac,
	}
}

func (i OUINic) KArgs() []string { return nil }

type UefiFw struct {
	Code, Vars string
}

func (u UefiFw) Cmdline() []string {
	return []string{
		"-drive", "if=pflash,format=raw,unit=0,readonly=on,file=" + u.Code,
		"-drive", "if=pflash,format=raw,unit=1,readonly=off,file=" + u.Vars,
		"-boot", "menu=on",
	}
}

func (u UefiFw) KArgs() []string { return nil }
func UefiFwSetup(tb gtst.TB, tmpdir string) *UefiFw {
	//copy vars file
	ovmfDir := os.Getenv("OVMF_DIR")
	if len(ovmfDir) == 0 {
		tb.Fatal("missing OVMF_DIR")
	}
	varSrc := fp.Join(ovmfDir, "OVMF_VARS.fd")
	varDst := fp.Join(tmpdir, "OVMF_VARS.fd")
	varData, err := ioutil.ReadFile(varSrc)
	if err != nil {
		tb.Fatal(err)
	}
	err = ioutil.WriteFile(varDst, varData, 0644)
	if err != nil {
		tb.Fatal(err)
	}
	return &UefiFw{
		Vars: varDst,
		Code: fp.Join(ovmfDir, "OVMF_CODE.fd"),
	}
}

type Bmc struct{}

func (b Bmc) Cmdline() []string {
	return []string{
		"-device", "ipmi-bmc-sim,id=bmc",
		"-device", "isa-ipmi-kcs,bmc=bmc",
	}
}

func (b Bmc) KArgs() []string { return nil }

type ArbitraryKArgs []string

func (a ArbitraryKArgs) Cmdline() []string { return nil }

func (a ArbitraryKArgs) KArgs() []string { return a }

type NoReboot struct{}

func (NoReboot) Cmdline() []string { return []string{"-no-reboot"} }
func (NoReboot) KArgs() []string   { return nil }

type UsbPassthrough struct {
	Hostbus, Hostport string
}

func (up *UsbPassthrough) Cmdline() []string {
	return []string{
		"-device",
		fmt.Sprintf("usb-host,hostbus=%s,hostport=%s", up.Hostbus, up.Hostport),
	}
}
func (up *UsbPassthrough) KArgs() []string { return nil }

//oob func
