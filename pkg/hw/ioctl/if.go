// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

//Package ioctl uses IOCTL's to find block device size, adjust network
//interface state, etc.
package ioctl

import (
	"syscall"
	"unsafe"
)

// https://golang.org/src/syscall/lsf_linux.go
type iflags struct {
	name  [syscall.IFNAMSIZ]byte
	flags uint16
}

func SetNicState(name string, up bool) error {
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM, 0)
	if err != nil {
		return err
	}
	defer syscall.Close(fd)
	var flags uint16
	flags, err = GetIfFlags(fd, name)
	if err != nil {
		return err
	}
	if up {
		flags |= uint16(syscall.IFF_UP)
	} else {
		flags &^= uint16(syscall.IFF_UP)
	}
	return SetIfFlags(fd, name, flags)
}

//return true if no errors and nic is up, false otherwise
func NicIsUp(name string) (up bool) {
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM, 0)
	if err != nil {
		return
	}
	defer syscall.Close(fd)
	var flags uint16
	flags, err = GetIfFlags(fd, name)
	if err != nil {
		return
	}
	up = (flags&uint16(syscall.IFF_UP) == uint16(syscall.IFF_UP))
	return
}

func GetIfFlags(fd int, name string) (uint16, error) {
	var ifl iflags
	copy(ifl.name[:], []byte(name))
	e := ioctl(uintptr(fd), uintptr(syscall.SIOCGIFFLAGS), uintptr(unsafe.Pointer(&ifl)))
	return ifl.flags, e
}

func SetIfFlags(fd int, name string, flags uint16) error {
	var ifl iflags
	copy(ifl.name[:], []byte(name))
	ifl.flags = flags
	return ioctl(uintptr(fd), uintptr(syscall.SIOCSIFFLAGS), uintptr(unsafe.Pointer(&ifl)))
}
