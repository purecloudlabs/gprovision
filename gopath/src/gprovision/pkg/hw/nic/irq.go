// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package nic

import (
	"fmt"
	"gprovision/pkg/hw/cpu"
	"gprovision/pkg/log"
	"io/ioutil"
	fp "path/filepath"
	"strconv"
	"strings"
)

//grep -E "^ *(CPU|$(ls /sys/class/net/en*/device/msi_irqs/|grep -v ^/|xargs|tr ' ' '|'))" /proc/interrupts
/*
           CPU0       CPU1       CPU2       CPU3
 46:         13          6        574          2  IR-PCI-MSI 409600-edge      eno1
 47:          0          0          1          0  IR-PCI-MSI 1048576-edge      enp2s0
 48:          8        834          6          1  IR-PCI-MSI 1048577-edge      enp2s0-TxRx-0
 49:         11          0        607          2  IR-PCI-MSI 1048578-edge      enp2s0-TxRx-1
 50:         10          3         10        601  IR-PCI-MSI 1048579-edge      enp2s0-TxRx-2
 51:       1702          2          4          0  IR-PCI-MSI 1048580-edge      enp2s0-TxRx-3


ls /sys/class/net/enp2s0/device/msi_irqs/
47 48 49 50 51
*/

/* NOTE NICs also have /sys/class/net/X/device/local_cpulist
   not sure whether only using local cpus would be better or worse...
   for now, assume it wouldn't be a significant improvement over using all
*/

//populate list of irqs which may fire on packet ingress, return number found
//on NICs without RSS, uses the main irq - otherwise, RxTx IRQs
func (nic *Nic) FindIRQs() (num int, err error) {
	//WARNING /sys/class/net/*/device/irq is NOT accurate, at least for mini's i218
	nic.irqs = nil
	msi := fp.Join(sysClassNet, nic.device, "device/msi_irqs/")
	d, err := ioutil.ReadDir(msi)
	if err != nil {
		return
	}
	for _, f := range d {
		var i uint64
		i, err = strconv.ParseUint(f.Name(), 10, 64)
		if err != nil {
			nic.irqs = nil
			return
		}
		nic.irqs = append(nic.irqs, i)
	}
	/* When there are multiple IRQs, we have RSS. With RSS, the main
	 *  IRQ fires extremely rarely - so we don't care about it
	 */
	if len(nic.irqs) > 1 {
		for n, i := range nic.irqs {
			if !IsQueueIrq(i) {
				nic.irqs = append(nic.irqs[:n], nic.irqs[n+1:]...)
				break
			}
		}
	}
	num = len(nic.irqs)
	return
}

//true if i appears to be the irq for a queue (based on irq name alone)
func IsQueueIrq(i uint64) bool {
	n := strings.ToLower(IrqName(i))
	//broadcom tg3: eno1-txrx-1
	//intel i350:   ens6f3-TxRx-0
	return strings.Contains(n, "rx") || strings.Contains(n, "tx")
}

//return list of device IRQs
func (nic Nic) ListIRQs() []uint64 {
	return nic.irqs
}

//return name of irq
func IrqName(i uint64) string {
	d, err := ioutil.ReadDir(fmt.Sprintf("/proc/irq/%d/", i))
	if err != nil {
		return ""
	}
	for _, e := range d {
		if e.IsDir() {
			return e.Name()
		}
	}
	return ""
}

//assign device's IRQs to the specified CPUs
func (nic Nic) AssignIRQs(cpus cpu.CpuSet) {
	for i, c := range cpus {
		//note, cpu list could be smaller than # of irq's if nic had a huge number of queues... unlikely though
		nic.bindIrqToCpu(nic.irqs[i], c)
	}
}

//write to /proc/irq/n/smp_affinity_list
//no benefit to providing a _range_, right?
func (nic Nic) bindIrqToCpu(irq uint64, c uint16) {
	file := fmt.Sprintf("/proc/irq/%d/smp_affinity_list", irq)
	err := ioutil.WriteFile(file, []byte(fmt.Sprintf("%d\n", c)), 0644)
	if err != nil {
		log.Logf("failed to set affinity to %d for %s:%d - %s\n", c, nic.device, irq, err)
	}
}
