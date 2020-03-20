// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package nic

import (
	"fmt"
	"testing"
)

//func XpsConfig(qnames []string, cpuMask uint64, nrCpus int) (qMasks map[string]uint64)
func TestXpsConfig(t *testing.T) {
	qnames := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i"}

	//nrCpus must be >= # of bits set in mask
	checkConf(t, qnames, 0xff, 8, 1)
	checkConf(t, qnames[:8], 0xff, 8, 2)
	checkConf(t, qnames, 0xff, 9, 3)
	checkConf(t, qnames, 0xff, 5, 4)
	checkConf(t, qnames[:8], 0xffff, 16, 5)
	checkConf(t, qnames[:8], 0xf5ae, 11, 6)
	checkConf(t, qnames[:8], 0xf5ae, 16, 7)
	checkConf(t, qnames[:5], 0xf5ae, 11, 8)
	checkConf(t, qnames[:5], 0xf5ae, 12, 9)
	checkConf(t, qnames[:8], 0xffff, 5, 10)
	checkConf(t, qnames[:8], 0xf5ae, 5, 11)
}

func countBits(m uint64) (bits uint16) {
	var i uint = 0
	for ; i < 64; i++ {
		if ((m >> i) & 1) == 1 {
			bits++
		}
	}
	return
}

func checkConf(t *testing.T, qnames []string, cpumask uint64, nrcpus uint16, testNumber int) {
	masks := XpsConfig(qnames, cpumask, nrcpus)
	desc := fmt.Sprintf("#%d(queues:%d/mask:%x/n:%d)", testNumber, len(qnames), cpumask, nrcpus)
	//t.Logf("conf %s masks %#v", desc, masks)

	var reassembledMask uint64
	for q, m := range masks {
		if m&reassembledMask != 0 {
			t.Errorf("%s: multiple masks contain bits in %s(%x). masks:\n%#v\n", desc, q, m, masks)
		}
		if m&cpumask != m {
			t.Errorf("%s: bits were added in %s(%x). masks:\n%#v\n", desc, q, m, masks)
		}
		reassembledMask |= m
	}
	bitsR := countBits(reassembledMask)
	if (reassembledMask != cpumask) && (bitsR < nrcpus) {
		t.Errorf("%s: mask mismatch - reassembled:\n%x , original:\n%x\nmasks:\n%#v\n", desc, reassembledMask, cpumask, masks)
	}
}
