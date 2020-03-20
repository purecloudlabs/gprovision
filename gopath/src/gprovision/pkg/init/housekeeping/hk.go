// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

//Package housekeeping works with lists of tasks to be performed in the future.
//Like defer, it is last-in first-out. Tasks can be removed from a list via
//filter functions, then assigning the filtered result to the list. To process
//the list of tasks, call Perform. Its bool arg indicates success/fail of the
//current process - for example, factory restore. Most tasks will ignore this
//bool, but some do use it. One example is the history package.
package housekeeping

import (
	"fmt"
	"gprovision/pkg/hw/cfa"
	"gprovision/pkg/log"
	"time"

	"golang.org/x/sys/unix"
)

type HkFun func(success bool)
type HkTask struct {
	Name string
	Func HkFun
	//DiscardOnUnwind bool
}
type HkList struct{ tasks []*HkTask }

type HkFilter func(t *HkTask) bool

//return subset of given list where filter matches (only positives)
func (hl *HkList) Filter(filter HkFilter) HkList {
	var out HkList
	for _, entry := range hl.tasks {
		if filter(entry) {
			out.tasks = append(out.tasks, entry)
		}
	}
	return out
}

//return subset of given list where filter does not match (remove positives)
func (hl *HkList) FilterOut(filter HkFilter) HkList {
	//simply invert the filter
	return hl.Filter(func(t *HkTask) bool { return !filter(t) })
}

func (hl *HkList) Perform(success bool) {
	//go through list, last first. Remove tasks as they are done.
	for {
		l := len(hl.tasks)
		if l == 0 {
			return
		}
		hl.tasks[l-1].Func(success)
		hl.tasks = hl.tasks[:l-1]
	}
}

func (hl *HkList) Clear() { hl.tasks = nil }

func (hl *HkList) Add(t *HkTask) {
	hl.tasks = append(hl.tasks, t)
}
func (hl *HkList) AddFirst(t *HkTask) {
	hl.tasks = append([]*HkTask{t}, hl.tasks...)
}

//Adds to the list functions to finish the log, unmount filesystems, and sync disks.
//These functions are always inserted at the beginning of the list.
//To avoid an import cycle, the unmount function must be passed in.
func AddPrebootDefaults(unmountFunc func(bool)) {
	// These must be the _last_ things run, so we add them at the beginning of
	// the list. Added in reverse order.
	RemovePrebootDefaults()
	Preboots.AddFirst(&HkTask{Name: "log.Finalize", Func: func(_ bool) { log.Finalize() }})
	Preboots.AddFirst(&HkTask{Name: "umount", Func: func(success bool) {
		if unmountFunc != nil {
			unmountFunc(success)
		}
	}})
	Preboots.AddFirst(&HkTask{Name: "sync", Func: func(_ bool) {
		fmt.Println("Flushing disk cache...")
		ss := time.Now()
		unix.Sync()
		fmt.Printf("sync: %s\n", time.Since(ss))
	}})
	Preboots.AddFirst(&HkTask{Name: "lcd", Func: func(_ bool) {
		cfa.DefaultLcd.Close()
		cfa.DefaultLcd = nil
	}})
}

func RemovePrebootDefaults() {
	Preboots = Preboots.FilterOut(func(t *HkTask) bool {
		switch t.Name {
		case "umount":
			return true
		case "sync":
			return true
		case "log.Finish":
			return true
		case "lcd":
			return true
		}
		return false
	})
}

var Preboots HkList
