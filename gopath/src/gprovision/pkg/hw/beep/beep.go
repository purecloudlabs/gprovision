// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

//Package beep activates PC Speaker on hardware supporting it.
package beep

import (
	"os"
	"syscall"
	"time"
)

//documentation in `man console_ioctl`

//Beep once. User must be able to open /dev/console.
func Beep() error {
	con, err := os.OpenFile("/dev/console", os.O_RDONLY, 0400)
	if err != nil {
		return err
	}
	defer con.Close()
	fd := con.Fd()
	return beep(fd)
}

func beep(fd uintptr) error {
	KDMKTONE := 0x4b30
	var val uint32 = (125 << 16) + 0x637 //duration and pitch
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(KDMKTONE), uintptr(val))
	if err == 0 {
		return nil
	}
	return err
}

//Beep until `done` is closed. User must be able to open /dev/console.
func BeepUntil(done chan struct{}, delay time.Duration) error {
	con, err := os.Open("/dev/console")
	if err != nil {
		return err
	}
	defer con.Close()
	fd := con.Fd()
	for {
		select {
		case <-done:
			return nil
		default:
		}
		e := beep(fd)
		if e != nil {
			return e
		}
		time.Sleep(delay)
	}
}
