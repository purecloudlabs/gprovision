// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package nic

import (
	"os/exec"
	"strconv"
	"strings"

	"github.com/purecloudlabs/gprovision/pkg/log"
)

type currMax struct {
	//values will never be very large, but using int64 reduces number of type conversions needed
	current, max int64
}
type chanCfgSection int

const (
	unknown chanCfgSection = iota
	max
	current
)

func (cm *currMax) read(section chanCfgSection, v string) {
	val, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		log.Logf("failed to parse ethtool output: %s for %s", err, v)
	}
	switch section {
	case max:
		cm.max = val
	case current:
		cm.current = val
	default:
		log.Logf("unknown section %d", section)
	}
}

type nicQueueCfg struct {
	rx, tx, other, combined currMax
}

/* MaximizeQueues - enable any NIC queues that were't automatically enabled.
 * Must happen before other queue-related operations so that the configuration
 * will apply to these additional queues. Intel seems to configure the max
 * queues automatically while Broadcom doesn't.
 */
func (nic *Nic) MaximizeQueues() {
	rssInfo := exec.Command("ethtool", "-l", nic.device)
	out, err := rssInfo.CombinedOutput()
	if err != nil {
		log.Logf("err running %#v: %s\noutput:%s\n", rssInfo.Args, err, out)
		return
	}
	config := parseQueueInfo(nic.device, out)
	/* Intel nics have combined rx-tx queues, while Broadcom uses separate queues
	 * Are there nics that support separate AND combined channels? If so, what
	 *   should be configured?
	 * For now, assume it's safe to set everything to max.
	 */
	nic.applyMax(config.rx, "rx")
	nic.applyMax(config.tx, "tx")
	nic.applyMax(config.other, "other")
	nic.applyMax(config.combined, "combined")
}

func parseQueueInfo(name string, out []byte) (info nicQueueCfg) {
	if strings.Contains(string(out), "Cannot get device channel parameters") {
		log.Logf("Device %s appears to have no queues", name)
		return
	}
	var section chanCfgSection
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		items := strings.Split(line, ":")
		if len(items) != 2 {
			log.Logf("%s: failed to parse ethtool output. problematic line:\n%s\nfull output:\n%s", name, line, out)
			continue
		}
		k := strings.TrimSpace(items[0])
		v := strings.TrimSpace(items[1])
		switch k {
		case "Pre-set maximums":
			section = max
		case "Current hardware settings":
			section = current
		case "RX":
			info.rx.read(section, v)
		case "TX":
			info.tx.read(section, v)
		case "Other":
			info.other.read(section, v)
		case "Combined":
			info.combined.read(section, v)
		default:
			if !strings.HasPrefix(strings.TrimSpace(items[0]), "Channel parameters for") {
				log.Logf("%s: failed to parse ethtool output. problematic line:\n%s\nfull output:\n%s", name, line, out)
				continue
			}
		}
	}
	return
}

func (nic *Nic) applyMax(cm currMax, id string) {
	if cm.current != cm.max {
		log.Logf("%s: adjusting %s channel from %d to %d", nic.device, id, cm.current, cm.max)
		set := exec.Command("ethtool", "--set-channels", nic.device, id, strconv.Itoa(int(cm.max)))
		out, err := set.CombinedOutput()
		if err != nil {
			log.Logf("err running %#v: %s\noutput:%s\n", set.Args, err, out)
		}
	}
}
