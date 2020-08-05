// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package nic

import (
	"fmt"
	"io/ioutil"

	"github.com/purecloudlabs/gprovision/pkg/log"
)

type queueMasks map[string]uint64

/* TODO take advantage of siblings information -
 * /sys/devices/system/cpu/cpuX/topology/core_siblings
 * /sys/devices/system/cpu/cpuX/topology/thread_siblings
 * /sys/devices/system/cpu/cpuX/cache/indexX/shared_cpu_map
 */

//Transmit Packet Scaling - set up cpumasks for the queues.
//We want a set of masks with no gaps or overlap.
//These IRQs must be excluded from the set irqbalance can change.
//
// Suggested XPS Configuration
//
// For a network device with a single transmission queue, XPS configuration
// has no effect, since there is no choice in this case. In a multi-queue
// system, XPS is preferably configured so that each CPU maps onto one queue.
// If there are as many queues as there are CPUs in the system, then each
// queue can also map onto one CPU, resulting in exclusive pairings that
// experience no contention. If there are fewer queues than CPUs, then the
// best CPUs to share a given queue are probably those that share the cache
// with the CPU that processes transmit completions for that queue
// (transmit interrupts).
//
// /sys/class/net/<dev>/queues/tx-<n>/xps_cpus
func XpsConfig(qnames []string, cpuMask uint64, nrCpus uint16) (qMasks queueMasks) {
	nrq := uint16(len(qnames))

	qMasks = make(map[string]uint64)

	cPerQ := nrCpus / nrq
	cRemaining := nrCpus % nrq
	if cPerQ == 0 {
		cPerQ = 1
		cRemaining = nrCpus - nrq
	}

	var i uint64 //index into system cpu bitmask
	for _, q := range qnames {
		var qCount uint16 = 0
		var qMask uint64 //cpu mask for this queue
		for qCount < cPerQ {
			if ((cpuMask >> i) & 1) == 1 {
				qMask |= (1 << i)
				qCount++
			}
			i++
			if i >= 64 {
				break
			}
		}
		qMasks[q] = qMask
	}
	//take care of any remainder
	//if cpu list is sparse, remaining cpus won't necessarily be added to sequential queues
	for cRemaining > 0 && i < 64 {
		for q, m := range qMasks {
			if ((cpuMask >> i) & 1) == 1 {
				m |= (1 << i)
				cRemaining--
			}
			qMasks[q] = m
			i++
			if i >= 64 {
				break
			}
		}
	}
	return
}

//write masks for queues
func WriteMasks(qMasks map[string]uint64, suffix string) (errors int) {
	for q, m := range qMasks {
		q += suffix
		err := ioutil.WriteFile(q, []byte(fmt.Sprintf("%x\n", m)), 0644)
		if err != nil {
			log.Logf("error writing %x to %s: %s\n", m, q, err)
			errors++
		}
	}
	return
}
