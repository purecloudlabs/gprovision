// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package integ

import (
	"flag"
	"fmt"
	"gprovision/pkg/common/strs"
	"gprovision/pkg/log"
	"gprovision/testing/vm"
	"os"
	"os/exec"
	"strings"

	"github.com/u-root/u-root/pkg/qemu"
)

//flags altering vm (from makefile):
//   bool
//uefi
//lcd
//local
//console
//sh
//   multi-value
//roothdd
//ipmi
//m
//flags
//emerg

//TODO test various combinations of the above

//gdb kernel debug?

//mock lcd?
//-device usb-serial,
//-device usb-serial:serial=vendorid=0x0403:productid=0x0102:tcp:192.168.0.2:4444
//-device usb-serial,chardev=tcpser -chardev udp,id=tcpser,
//-device usb-serial,chardev=tcpser -chardev socket,id=tcpser,host=10.0.0.1,port=9000,server

//see flags below for descriptions
var (
	Uefi      bool
	Lcd       bool
	Keep      bool
	M         int
	Img       string
	Tmp       string
	KDir      string
	KOverride bool
	P9        bool

//	Sh        bool //unused
// Console bool
// Dry     bool
)

func Flags() {
	// flag.BoolVar(&Sh, "sh", false, "drop to mfg shell")
	// flag.BoolVar(&Console, "console", true, "false for window/graphics")
	// flag.BoolVar(&Dry, "dry", false, "dry run - print args, don't run qemu")
	flag.BoolVar(&Uefi, "uefi", Uefi, "uefi fw rather than legacy")
	flag.BoolVar(&Lcd, "lcd", false, "lcd passthrough")
	flag.IntVar(&M, "mem", 0, "memory in megs; default is 512, or 5120 if -img")
	flag.BoolVar(&Keep, "keep", false, "do not delete temp dir")
	flag.StringVar(&Tmp, "tmp", "", "override temp dir")
	flag.StringVar(&Img, "img", "", "use real image rather than fake. Increases mem.")
	flag.StringVar(&KDir, "kdir", "", "kernel dir, defaults to REPO_ROOT/work")
	flag.BoolVar(&KOverride, "ko", false, "override kernel version logic - use "+strs.BootKernel()+" from -kdir")
	flag.BoolVar(&P9, "p9", false, "tmpfs via 9p; logs will not show on console")
}

const GB = 1024 * 1024 * 1024

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

//Return true if running on CI. Currently only detects Jenkins. For cleanup.
func OnCI() bool {
	for _, e := range []string{
		"JENKINS_NODE_COOKIE",
	} {
		if _, present := os.LookupEnv(e); present {
			return true
		}
	}
	return false
}

func PostMfgFixups(qopts *qemu.Options) {
	//mfg runs qemu with specified kernel, but subsequent boots use kernel
	// installed in vm - so we do some cleanup
	var devs []qemu.Device
	for _, d := range qopts.Devices {
		if blk, ok := d.(*vm.BlockDev); ok {
			blk.MustExist = true
			if blk.Id == "recovery" {
				//tells qemu to give this drive boot priority
				blk.BootIndex = 1
			}
		}
		_, ok := d.(vm.ArbitraryKArgs)
		if ok {
			//remove this from the list
			//next steps do not use external kernel, so we can't specify kernel args
			continue
		}
		devs = append(devs, d)
	}
	qopts.Devices = devs

	qopts.Kernel = ""
	qopts.Initramfs = ""
	qopts.KernelArgs = ""
}

//fixes up qemu args to work for windowed display
func WindowedVMCmd(opts *qemu.Options) *exec.Cmd {
	args, err := opts.Cmdline()
	if err != nil {
		log.Fatalf("cmdline: %s", err)
	}
	q := exec.Command(args[0])
	for _, a := range args[1:] {
		if a == "-nographic" {
			continue
		}
		q.Args = append(q.Args, a)
	}
	q.Args = append(q.Args, "-usb", "-usbdevice", "tablet")
	if opts.SerialOutput != nil {
		q.Stdout = opts.SerialOutput
	}
	return q
}

//pretty-prints (quotes) args for user re-use.
func ExecQuoted(xargs []string) string {
	args := make([]string, len(xargs))
	for i, arg := range xargs {
		if strings.ContainsAny(arg, " \t\n") {
			args[i] = fmt.Sprintf("%q", arg)
		} else {
			args[i] = arg
		}
	}
	return strings.Join(args, " ")
}
