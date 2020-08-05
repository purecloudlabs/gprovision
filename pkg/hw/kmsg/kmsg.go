// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

// Package kmsg facilitates processes writing to the kernel ring buffer.
// Process must run as root.
package kmsg

import (
	"fmt"
	"io"
	"os"

	"github.com/purecloudlabs/gprovision/pkg/log"
)

type Priority uint

//Convert facility/severity into priority
func Prio(f Facility, s Severity) Priority {
	return Priority(f*8) + Priority(s)
}

//Facility values a la RFC5424. Incomplete list.
type Facility uint

const (
	FacUser   Facility = 1
	FacSys             = 3
	FacSec             = 4
	FacLocal0          = 16
)

//Severity values a la RFC5424. Incomplete list.
type Severity uint

const (
	SevEmerg Severity = iota
	SevAlert
	SevCrit
	SevError
	SevWarn
	SevNotice
)

var defaultPrio = &KmsgWithPrio{prio: Prio(FacUser, SevNotice)}

// Printf writes to /dev/kmsg _and_ stderr. kmsg is not kept open - not intended
// for frequent use.
func Printf(f string, va ...interface{}) { defaultPrio.Printf(f, va...) }

type KmsgWithPrio struct {
	f    io.WriteCloser
	prio Priority
	pfx  string
}

func NewKmsgPrio(f Facility, s Severity, pfx string) *KmsgWithPrio {
	if f == 0 {
		fmt.Fprintln(os.Stderr, "cannot use facility 0")
		return nil
	}
	kmsg := openKmsg()
	if kmsg == nil {
		return nil
	}
	kp := &KmsgWithPrio{
		prio: Prio(f, s),
		f:    kmsg,
		pfx:  pfx,
	}
	return kp
}

// Printf writes to /dev/kmsg _and_ stderr.
func (km *KmsgWithPrio) Printf(f string, va ...interface{}) {
	var msg string
	if km != nil {
		msg = fmt.Sprintf("<%d>", km.prio)
		if len(km.pfx) > 0 {
			msg += km.pfx + ": "
		}
	}

	msg += fmt.Sprintf(f, va...)
	fmt.Fprintln(os.Stderr, msg)
	km.write(msg)
}

// Like Printf, but writes to kmsg and then via package github.com/purecloudlabs/gprovision/pkg/log - does not
// write to stdout/stderr itself, to avoid output duplication
func (km *KmsgWithPrio) Logf(f string, va ...interface{}) {
	var msg string
	if km != nil {
		msg = fmt.Sprintf("<%d>", km.prio)
		if len(km.pfx) > 0 {
			msg += km.pfx + ": "
		}
	}
	msg += fmt.Sprintf(f, va...)
	km.write(msg)
	log.Logf(f, va...)
}

func (km *KmsgWithPrio) write(msg string) {
	if km != nil {
		kmsg := km.f
		if kmsg == nil {
			kmsg = openKmsg()
			if kmsg != nil {
				defer kmsg.Close()
			}
		}
		if kmsg != nil {
			fmt.Fprint(kmsg, msg)
		}
	}
}

func (km *KmsgWithPrio) Close() error {
	err := km.f.Close()
	km.f = nil
	return err
}

func openKmsg() *os.File {
	kmsg, err := os.OpenFile("/dev/kmsg", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open /dev/kmsg: %s\n", err)
		return nil
	}
	return kmsg
}
