// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package cpu

import (
	"fmt"
	"gprovision/pkg/log"
	"io/ioutil"
	"os"
	fp "path/filepath"
	"strconv"
	"strings"
)

/* read siblings information -
 * /sys/devices/system/cpu/cpuX/topology/core_siblings_list
 * /sys/devices/system/cpu/cpuX/topology/thread_siblings_list
 * and use it to determine which cpus to use for a given set of IRQs (CpuSet)
 */

/* could also use cache info... would it ever tell us anything useful that core/thread sibs didn't?
 * /sys/devices/system/cpu/cpuX/cache/indexX/shared_cpu_list
 */

//CpuSet: a set of cpus
//generally linked in some way that affects performance
type CpuSet []uint16

//map from cpu to list of siblings
type topo map[uint16]CpuSet

const (
	sysCpu = "/sys/devices/system/cpu"
	cS     = "topology/core_siblings_list"
	tS     = "topology/thread_siblings_list"
)

var allCpus CpuSet
var coreSib topo //from core_siblings_list, but perhaps better to think of them as socket siblings
var threadSib topo
var useCount map[uint16]int

func CoreSiblings() topo {
	return coreSib
}
func ThreadSiblings() topo {
	return threadSib
}
func AllCpus() CpuSet {
	//populate allCpus on first use, unless this is a test
	if len(allCpus) == 0 && !strings.HasSuffix(os.Args[0], ".test") {
		populateMaps()
	}
	return allCpus
}

func init() {
	coreSib = make(topo)
	threadSib = make(topo)
	useCount = make(map[uint16]int)
}

func populateMaps() {
	allCpus = getCpuList(fp.Join(sysCpu, "online"))
	for _, c := range allCpus {
		//map cores to their socket siblings
		cs := getCpuList(fp.Join(sysCpu, fmt.Sprintf("cpu%d", c), cS))
		coreSib[c] = cs
		//map cores to their thread siblings
		ts := getCpuList(fp.Join(sysCpu, fmt.Sprintf("cpu%d", c), tS))
		threadSib[c] = ts
	}
}
func fatalIf(e error, i ...interface{}) {
	if e == nil {
		return
	}
	s := "error"
	if len(i) > 0 {
		f := i[0].(string)
		if len(i) > 1 {
			s = fmt.Sprintf(f, i[1:]...)
		} else {
			s = f
		}
	}
	panic(fmt.Sprintf("%s: %s", s, e))
}

func getCpuList(f string) (s CpuSet) {
	c, err := ioutil.ReadFile(f)
	fatalIf(err)
	s, err = parseCpuList(c)
	fatalIf(err)
	return
}

//parse a cpulist as used by the kernel
//ex. '0-7,45-49,52'
func parseCpuList(lst []byte) (s CpuSet, err error) {
	parts := strings.Split(strings.Trim(string(lst), "\n"), ",")
	for _, p := range parts {
		if strings.ContainsRune(p, '-') {
			cRange := strings.Split(p, "-")
			if len(cRange) != 2 {
				panic("bad len")
			}
			var lowerBound, upperBound uint64
			lowerBound, err = strconv.ParseUint(cRange[0], 10, 16)
			if err != nil {
				return
			}
			upperBound, err = strconv.ParseUint(cRange[1], 10, 16)
			if err != nil {
				return
			}
			if upperBound <= lowerBound {
				err = fmt.Errorf("bad range '%s' in '%s'", p, string(lst))
				return
			}
			for i := lowerBound; i <= upperBound; i++ {
				s = append(s, uint16(i))
			}
		} else {
			var val uint64
			val, err = strconv.ParseUint(p, 10, 16)
			if err != nil {
				return
			}
			for i, v := range s {
				if uint16(val) == v {
					err = fmt.Errorf("duplicate value %d at %d,%d in '%s'", val, i, len(s), string(lst))
					return
				}
			}
			s = append(s, uint16(val))
		}
	}
	return
}

func (s CpuSet) Contains(c uint16) bool {
	for _, v := range s {
		if c == v {
			return true
		}
	}
	return false
}

//check for thread siblings
func (s CpuSet) ContainsTSib(c uint16) bool {
	siblings := threadSib[c]
	for _, v := range s {
		if siblings.Contains(v) && c != v {
			return true
		}
	}
	return false
}

//check for core siblings
func (s CpuSet) ContainsCSib(c uint16) bool {
	siblings := coreSib[c]
	for _, v := range s {
		if siblings.Contains(v) && c != v {
			log.Logf("%v contains %d, sibling of %d in %v\n", s, v, c, siblings)
			return true
		}
	}
	return false
}

//weighting function to determine next best core
//useCount tracks exact core use, add to that .5 for each used tsib and .1 per csib
func weight(c uint16) (w float64) {
	siblingWeight := func(siblings topo, c uint16) (sw int) {
		for _, s := range siblings[c] {
			if s == c {
				continue
			}
			sw += useCount[s]
		}
		return
	}
	w = float64(useCount[c])
	w += float64(siblingWeight(threadSib, c)) * .5
	w += float64(siblingWeight(coreSib, c)) * .01
	return
}

//uses weight() to find lightest cpu not in exclude
func lowestWeight(exclude CpuSet) (lowest uint16) {
	if uint16(len(useCount)) < numCpus {
		for _, c := range allCpus {
			useCount[c] = 0
		}
	}
	first := true
	wl := weight(lowest)
	for c := range useCount {
		if exclude.Contains(c) {
			continue
		}
		wc := weight(c)
		if first || wc < wl {
			lowest = c
			wl = wc
			first = false
		}
	}
	return
}

//keep track of how many queues are on each core
//when creating set, prefer unassigned cores/sockets
func CreateSetWeighted(setSize int) (set CpuSet) {
	size := uint16(setSize)
	if size > numCpus {
		size = numCpus
	}
	for uint16(len(set)) < size {
		next := lowestWeight(set)
		useCount[next] += 1
		set = append(set, next)
	}
	return
}
