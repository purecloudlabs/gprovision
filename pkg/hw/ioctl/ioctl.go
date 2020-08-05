// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package ioctl

import (
	"syscall"
	"unsafe"
)

/*********
 * IMPORTANT
 * An ioctl() request has encoded in it whether the argument is an in
 *   parameter or out parameter, and the size of the argument argp in
 *   bytes.
 *********/

type FDer interface {
	Fd() uintptr
}

func Ioctl1(fd uintptr, cmd int) (res uint64, err error) {
	ptr := uintptr(unsafe.Pointer(&res))
	err = ioctl(fd, uintptr(cmd), ptr)
	return res, err
}

/* this is broken - a pointer to an interface won't work
func Ioctl2(fd uintptr, cmd int, data interface{}) (err error) {
	ptr := uintptr(unsafe.Pointer(&data))
	err = ioctl(fd, uintptr(cmd), ptr)
	return
} */

func ioctl(fd, cmd, ptr uintptr) error {
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL, fd, cmd, ptr)
	if err == 0 {
		return nil
	}
	return err
}
