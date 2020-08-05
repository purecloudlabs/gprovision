// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

// +build !release

package serial

import (
	"fmt"
	"io"
	"os"
	"reflect"
)

// Output for tracing, not present in release builds. Enable by assigning a
// writer - a file, bytes.Buffer, stderr, etc.
var Output io.Writer

// If output is non-nil, tracing occurs as functions return. If TraceEnter is
// also true, functions will trace upon entry.
var TraceEnter = false

//If exit is true, sets Output to os.Stderr, nil if false; sets TraceEnter to value of in.
func Debug(exit, enter bool) {
	if exit {
		Output = os.Stderr
	} else {
		Output = nil
	}
	TraceEnter = enter
}

// Trace enter/exit with args. To capture return values, you must pass pointers
// unless it's a slice. Assumes last arg to returned func is of type error. Not
// present in release builds.
//
//ex: defer tracef("Read(b)")(" [b=%q]  =(%d,%s)", b, &n, &err)
// (defer'd call to the function returned by tracef)
func tracef(f string, va ...interface{}) func(rfmt string, vb ...interface{}) {
	if Output == nil {
		return func(string, ...interface{}) {}
	}
	callStr := fmt.Sprintf(f, va...)
	retStr := callStr
	if TraceEnter {
		fmt.Fprintf(Output, ">  %s\n", callStr)
		retStr = " < " + callStr
	}
	return func(rfmt string, vb ...interface{}) {
		vn := len(vb)
		estr := "<nil>"
		for i, v := range vb {
			t := reflect.TypeOf(v)
			if t == nil {
				continue
			}
			switch t.Kind() {
			case reflect.Ptr:
				vb[i] = reflect.ValueOf(v).Elem().Interface()
			}
		}
		if vn > 0 {
			if vb[vn-1] != nil {
				estr = `"` + vb[vn-1].(error).Error() + `"`
			}
			vb[vn-1] = estr
		}
		fmt.Fprintf(Output, retStr+rfmt+"\n", vb...)
	}
}
