// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

// Service nic_config configures RFS, RSS, XPS for interfaces. Outputs IRQ
// ban list for irqbalancer. See gprovision/pkg/hw/cpu, gprovision/pkg/hw/nic.
package main

import (
	"fmt"
	"gprovision/pkg/appliance"
	"gprovision/pkg/hw/cpu"
	"gprovision/pkg/hw/nic"
	"gprovision/pkg/log"
	"gprovision/pkg/log/flags"
	"os"
)

/* we need to set IRQs ourselves. to do this, we need to get
 * irq #'s to irqbalance - and that means we must run first.
 * get irq's to irqbalance by writing a systemd override file
 * to /run, setting the IRQBALANCE_ARGS env var on every boot
 */

// IMPORTANT - read https://www.kernel.org/doc/Documentation/networking/scaling.txt

var buildId string

func main() {
	log.AddConsoleLog(flags.NA)
	log.FlushMemLog()

	fmt.Printf("build info: %s\n", buildId)
	platform := appliance.Read()
	if platform == nil {
		fmt.Fprintf(os.Stderr, "unknown platform\n")
		os.Exit(1)
	}
	diags := platform.DiagPorts()
	if len(diags) > 0 && nic.NICsAlreadyConfigured() {
		fmt.Fprintf(os.Stderr, "already ran, exiting without writing file or disabling any ports\n")
		os.Exit(0)
	}
	/* Diag ports must be disabled on some platforms. We do this by
	   disabling the n-th port, so running multiple times will
	   disable multiple ports. Thus the above check.
	*/
	prefixes := platform.MACPrefixes()
	for _, diag := range diags {
		success := nic.DisableByIndex(diag, prefixes)
		if !success {
			fmt.Printf("failed to disable DIAG #%d\n", diag)
		}
	}
	nics := nic.List()
	if nics == nil || len(nics) == 0 {
		panic("no nics!")
	}
	fmt.Printf("NICs found: %v\n", nics)
	/* While aliases are written to systemd-networkd files, systemd-udevd
	   is actually what reads those aliases from the files and sets them.
	   It doesn't seem to be possible to update the files and then cause
	   interface aliases to be sync'd to the files. So, load the aliases
	   and, if a nic is missing an alias, set it.
	*/
	aliases := platform.DefaultPortNames()
	allowedPrefixes := platform.MACPrefixes()
	for i, n := range nics.FilterMACs(allowedPrefixes).Sort() {
		if i >= len(aliases) {
			fmt.Printf("more interfaces than aliases - skipping %s (#%d)\n", n, i)
			continue
		}
		if n.SetAlias(aliases[i], false) {
			fmt.Printf("set alias for %s\n", n)
		}
	}

	mask := cpu.Mask()

	fmt.Printf("CPU Mask: 0x%x\nConfiguring RSS, RFS, and XPS for each NIC...\n", mask)
	var bannedIRQs []uint64
	for _, n := range nics {
		n.MaximizeQueues() //must happen first as this may enable additional queues that will need configured
		n.RssConfig()
		n.RfsConfig()
		txqNames := n.Queues("tx-")
		txqMasks := nic.XpsConfig(txqNames, mask, cpu.Count())
		nic.WriteMasks(txqMasks, "/xps_cpus")
		bannedIRQs = append(bannedIRQs, n.ListIRQs()...)
	}
	nic.WriteIRQBalanceBans(bannedIRQs)
}
