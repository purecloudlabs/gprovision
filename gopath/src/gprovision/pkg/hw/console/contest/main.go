// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

//Test app for console package; normal unit test won't work as the vt commands require root privs.
package main

import (
	"fmt"
	"time"
)

import (
	"gprovision/pkg/hw/console"
)

func main() {
	fmt.Println("message on current console")
	ch, err := console.MessageChannel()
	if err != nil {
		panic(err)
	}
	ch.Printf("message")
	time.Sleep(time.Second * 4)
	ch.Printf("message+4")
	time.Sleep(time.Second * 2)
	ch.Printf("message+6")
	fmt.Println("6")
	time.Sleep(time.Second * 6)
	fmt.Println("12")
	ch.Printf("message+12")
	time.Sleep(time.Millisecond * 10)
	ch.Close()
	time.Sleep(time.Second / 10)
	fmt.Println("back to orig console")
}
