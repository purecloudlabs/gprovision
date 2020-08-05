// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package nic

import (
	"os/exec"

	"github.com/purecloudlabs/gprovision/pkg/hw/cpu"
	"github.com/purecloudlabs/gprovision/pkg/log"
)

var rssProtos = []string{"tcp4", "tcp6", "udp4", "udp6", "sctp4", "sctp6"}

/* Receive Side Scaling - set rx flow hashes.
 * Doesn't affect IRQ affinity - we handle that elsewhere
 * rather than letting irqbalance have its way. Irqbalance
 * seems to not be locality-aware; we let it shuffle other
 * interrupts while we focus on NICs.
 */
func (nic *Nic) RssConfig() {
	//set rx flow hash to use src/dest ip (=sd), src/dest port (=fn)
	for _, p := range rssProtos {
		ethtool := exec.Command("ethtool", "-N", nic.device, "rx-flow-hash", p, "sdfn")
		out, err := ethtool.CombinedOutput()
		if err != nil {
			log.Logf("err running %#v: %s\noutput:%s\n", ethtool.Args, err, out)
		}
	}

	//adaptive rate for RSS IRQs
	//most or all of our hardware doesn't support this, but trying hurts nothing
	//so, attempt to turn it on but ignore any errors
	adaptiveRx := exec.Command("ethtool", "-C", nic.device, "adaptive-rx", "on")
	log.Cmd(adaptiveRx)

	//spread across cores/sockets
	num, err := nic.FindIRQs()
	if err != nil {
		log.Logf("err getting IRQs for NIC %s: %s\n", nic.device, err)
	}
	cpus := cpu.CreateSetWeighted(num)
	nic.AssignIRQs(cpus)
}
