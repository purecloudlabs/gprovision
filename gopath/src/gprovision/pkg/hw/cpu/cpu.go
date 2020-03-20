// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

//Package cpu allows determining topological arrangement of CPU cores, generally
//for use in setting RSS IRQs.
package cpu

import (
	"fmt"
	"gprovision/pkg/log"
	"io/ioutil"
	"strconv"
	"strings"
)

var numCpus uint16
var cpuMask uint64

func Count() uint16 {
	if numCpus == 0 {
		setCPUInfo()
	}
	return numCpus
}

func Mask() uint64 {
	if numCpus == 0 {
		setCPUInfo()
	}
	return cpuMask
}

/* set nrCpus, cpuMask (a bitmask of available CPUs)
 * runtime.NumCPU() won't always show what we want - the cpu list could
 * be sparse due to disabled/banned cpus, possibly other reasons
 * this uses /sys/devices/system/cpu/ to figure out what cpus exist, under
 * the assumption that it could be more complete than the info returned by the
 * function underlying NumCPU, sched_getaffinity.
 */
func setCPUInfo() {
	var mask uint64
	var count uint16
	files, err := ioutil.ReadDir("/sys/devices/system/cpu/")
	if err != nil {
		log.Logf("err %s reading dir for cpu mask\n", err)
		return
	}
	for _, fi := range files {
		name := fi.Name()
		if strings.HasPrefix(name, "cpu") && len(name) > 3 && isDigit(name[3]) {
			num, err := strconv.ParseUint(name[3:], 10, 64)
			if err == nil && num > 63 {
				err = fmt.Errorf("cpu index %d outside range of 64-bit mask", num)
			}
			if err != nil {
				log.Logf("err '%s' generating cpu mask at %s\n", err, name)
				continue
			}
			mask |= 1 << num
			count += 1
		}
	}
	cpuMask = mask
	numCpus = count
}

func isDigit(d uint8) bool {
	if d >= '0' && d <= '9' {
		return true
	}
	return false
}
