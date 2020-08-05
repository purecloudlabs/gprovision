// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package integ

import (
	"fmt"
	"os"
	fp "path/filepath"
	"strings"

	"github.com/purecloudlabs/gprovision/pkg/hw/cfa"
	"github.com/purecloudlabs/gprovision/pkg/log"
	"github.com/purecloudlabs/gprovision/testing/vm"

	"github.com/u-root/u-root/pkg/qemu"
)

//Finds lcd info necessary for passthrough to qemu vm.
//Tested with ftdi devices, not with cdc acm - may need some work there.
func GetLcd() (qemu.Device, error) {
	const sct = "/sys/class/tty"
	devs := cfa.FindDevs()
outer:
	for _, d := range devs {
		sysClassPath := fp.Join(sct, fp.Base(d)) // /sys/class/tty/ttyUSB0
		if exists(sysClassPath) {
			//found it
			sysDevPath, err := fp.EvalSymlinks(fp.Join(sysClassPath, "device"))
			if err != nil {
				log.Logf("%s: EvalSymlinks error %s", sysClassPath, err)
				continue outer
			}
			for len(sysDevPath) > 0 {
				//remove trailing elements from sysDevPath until we find "port" symlink
				sysDevPath = fp.Dir(sysDevPath)
				portSym := fp.Join(sysDevPath, "port")
				if exists(portSym) {
					// /sys/class/tty/ttyUSB0/device/../../port -> ../3-1:1.0/3-1-port3
					link, err := os.Readlink(portSym)
					if err != nil {
						log.Logf("%s: Readlink error %s", sysClassPath, err)
						continue outer
					}
					port := fp.Base(link)
					elements := strings.Split(port, "-")
					if len(elements) != 3 || !strings.Contains(elements[2], "port") {
						log.Logf("%s: unable to parse port in %s", sysDevPath, port)
						continue outer
					}
					return &vm.UsbPassthrough{
						Hostbus:  elements[0],
						Hostport: elements[1] + "." + strings.TrimPrefix(elements[2], "port"),
					}, nil
				}
			}
		}
	}
	return nil, fmt.Errorf("No lcd found")
}
