// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

//Package serial configures a serial port for use with Crystalfontz LCDs. Only implemented for linux.
package serial

import (
	"os"
	"unsafe"

	"golang.org/x/sys/unix"
)

type Port struct {
	f *os.File
}

func Open(dev string) (*Port, error) {
	f, err := os.OpenFile(dev, unix.O_RDWR|unix.O_NOCTTY|unix.O_NONBLOCK, 0666)
	if err != nil {
		return nil, err
	}
	p := &Port{f: f}

	opts, err := p.TcGetAttr()
	if err != nil {
		p.f.Close()
		return nil, err
	}

	//flags mostly from i3lcd/serial_port.cpp

	//input modes
	opts.Iflag = opts.Iflag &^ (unix.IGNBRK | unix.BRKINT | unix.PARMRK | unix.INPCK | unix.ISTRIP | unix.INLCR | unix.IGNCR | unix.ICRNL | unix.IXON | unix.IXOFF)
	opts.Iflag |= unix.IGNPAR

	//output modes
	opts.Oflag = opts.Oflag &^ (unix.OPOST | unix.ONLCR | unix.OCRNL | unix.ONOCR | unix.ONLRET | unix.OFILL | unix.OFDEL | unix.NLDLY | unix.CRDLY | unix.TABDLY | unix.BSDLY | unix.VTDLY | unix.FFDLY)

	//control modes
	/* unix.CSTOPB - i3lcd uses two stop bits and it seems to work, though the docs say 1 stop bit...?! */
	opts.Cflag = opts.Cflag &^ (unix.CSIZE | unix.PARENB | unix.PARODD | unix.HUPCL | unix.CRTSCTS | unix.CBAUD | unix.CSTOPB)
	opts.Cflag |= unix.CREAD | unix.CS8 | unix.CLOCAL
	opts.Cflag |= unix.B115200

	//local modes
	opts.Lflag = opts.Lflag &^ (unix.ISIG | unix.ICANON | unix.IEXTEN | unix.ECHO | unix.ECHOE | unix.ECHOK | unix.ECHOCTL | unix.ECHOKE)

	//clear cc
	for i := range opts.Cc {
		opts.Cc[i] = 0
	}
	//blocking read: VTIME = 0, VMIN = 1
	opts.Cc[unix.VMIN] = 1

	opts.Ispeed = unix.B115200
	opts.Ospeed = unix.B115200

	err = p.TcSetAttr(opts)
	if err != nil {
		p.f.Close()
		return nil, err
	}
	err = unix.SetNonblock(int(p.f.Fd()), false)
	if err != nil {
		p.f.Close()
		return nil, err
	}
	p.Flush()
	return p, nil
}

func (p *Port) TcGetAttr() (*unix.Termios, error)  { return TcGetAttr(p.f.Fd()) }
func (p *Port) TcSetAttr(opts *unix.Termios) error { return TcSetAttr(p.f.Fd(), opts) }
func (p *Port) Close() error                       { return p.f.Close() }

func (p *Port) Flush() (err error) {
	defer tracef("Flush()")("=%s", &err)
	return Flush(p.f.Fd())
}

func (p *Port) Read(b []byte) (n int, err error) {
	defer tracef("Read(b)")(" [b=%q]  =(%d,%s)", b, &n, &err)
	return p.f.Read(b)
}

func (p *Port) Write(b []byte) (n int, err error) {
	defer tracef("Write(%q)", b)("=(%d,%s)", &n, &err)
	return p.f.Write(b)
}

func TcGetAttr(fd uintptr) (*unix.Termios, error) {
	opts := &unix.Termios{}
	_, _, errno := unix.Syscall6(unix.SYS_IOCTL, fd, unix.TCGETS, uintptr(unsafe.Pointer(opts)), 0, 0, 0)
	if errno != 0 {
		return nil, errno
	}
	return opts, nil

}

func TcSetAttr(fd uintptr, opts *unix.Termios) error {
	_, _, errno := unix.Syscall6(unix.SYS_IOCTL, fd, unix.TCSETS, uintptr(unsafe.Pointer(opts)), 0, 0, 0)
	if errno != 0 {
		return errno
	}
	return nil
}

func Flush(fd uintptr) error {
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, fd, unix.TCFLSH, unix.TCIOFLUSH)
	if errno != 0 {
		return errno
	}
	return nil
}
