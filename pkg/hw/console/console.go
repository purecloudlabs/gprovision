// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

// Package console allows for some trivial manipulations of linux virtual
// terminals, such as switching which one is in the foreground, displaying
// messages, then switching back.
//
// Makes use of the system's fgconsole, chvt, deallocvt tools.
package console

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type VTid int64 //0 for unknown/error, -1 for serial

type vtChan struct {
	ch chan string
	ok bool
}

func (vt *vtChan) Printf(f string, va ...interface{}) {
	s := fmt.Sprintf(f, va...)
	fmt.Println(s)
	if vt.ok {
		vt.ch <- s
	}
}

func (vt *vtChan) Close() {
	if vt.ok {
		vt.ok = false
		close(vt.ch)
	}
}

//switch to an unused vt and display messages as they arrive in ch.
//destroy vt and switch back to original when ch is closed.
//NOTE - prog must not exit immediately after closing channel, or switch will not happen
func MessageChannel() (vt vtChan, err error) {
	vt.ch = make(chan string)
	var origVt, newVt VTid
	origVt, err = CurrentVt()
	if err != nil {
		return
	}
	if origVt == -1 {
		//serial - no point switching, as the user can only see messages on serial
		newVt = -1
	} else {
		newVt, err = UnusedVt()
		if err != nil {
			return
		}
		err = SetVt(newVt)
		if err != nil {
			return
		}
	}
	go func() {
		for l := range vt.ch {
			err := WriteVt(newVt, l)
			if err != nil {
				fmt.Println("Writing to console: ", err)
			}
		}
		//cleanup
		err := SetVt(origVt)
		if err != nil {
			fmt.Println("Switching to original console: ", err)
		}
		err = FreeVt(newVt)
		if err != nil {
			fmt.Println("Discarding temporary console: ", err)
		}
	}()
	vt.ok = true
	return
}

//get current console id
func CurrentVt() (id VTid, err error) {
	var out []byte
	out, err = exec.Command("fgconsole").CombinedOutput()
	if err != nil {
		return
	}
	str := strings.TrimSpace(string(out))
	if str == "serial" {
		id = -1
		return
	}
	var i uint64
	i, err = strconv.ParseUint(str, 10, 64)
	if err == nil {
		id = VTid(i)
	}
	return
}

//get next unused console id
func UnusedVt() (id VTid, err error) {
	var out []byte
	out, err = exec.Command("fgconsole", "--next-available").CombinedOutput()
	if err != nil {
		return
	}
	str := strings.TrimSpace(string(out))
	var i uint64
	i, err = strconv.ParseUint(str, 10, 64)
	if err == nil {
		id = VTid(i)
	}
	return
}

//switch to given vt
func SetVt(id VTid) error {
	if id == -1 {
		return nil
	}
	return exec.Command("chvt", fmt.Sprintf("%d", id)).Run()
}

func FreeVt(id VTid) error {
	if id == -1 {
		return nil
	}
	return exec.Command("deallocvt", fmt.Sprintf("%d", id)).Run()
}

//write line to given vt
func WriteVt(id VTid, s string) error {
	if id < 0 {
		id = 0
	}
	tty := fmt.Sprintf("/dev/tty%d", id)
	dev, err := os.OpenFile(tty, os.O_APPEND|os.O_WRONLY, 0400)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(dev, s)
	return err
}
