// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package ipmi

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/purecloudlabs/gprovision/pkg/common/rkeep"
	"github.com/purecloudlabs/gprovision/pkg/log"
)

//log ipmi macs
func LogMacs() {
	var macs []string
	chs := GetChannels()
	for _, c := range chs {
		if c.IsLan() {
			mac, err := c.Mac()
			if err != nil {
				log.Fatalf("Getting IPMI MACs: error %s", err)
			}
			macs = append(macs, mac)
		}
	}
	rkeep.StoreIPMIMACs(macs)
}

func (ch *Channel) Mac() (mac string, err error) {
	//ipmitool lan print 1
	p := exec.Command("ipmitool", "lan", "print", strconv.Itoa(ch.Id))
	return ch.mac(log.Cmd(p))
}

func (ch *Channel) mac(res string, success bool) (mac string, err error) {
	if !success {
		log.Fatalf("IPMI channel #%d: failed to get lan info", ch.Id)
	}
	lines := strings.Split(res, "\n")
	for _, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "MAC Address") {
			parts := strings.SplitN(l, ":", 2)
			if len(parts) != 2 {
				log.Logf("IPMI channel #%d: failed to parse line %s in ipmitool output\n%s\n", ch.Id, l, res)
				continue
			}
			mac = strings.TrimSpace(parts[1])
			if strings.Count(mac, ":") != 5 {
				log.Logf("IPMI channel #%d: doesn't look like a mac: '%s' from  %s", ch.Id, mac, l)
				mac = ""
				continue
			}
		}
	}
	if mac == "" {
		err = fmt.Errorf("IPMI channel #%d: no mac found in ipmitool output\n%s\n", ch.Id, res)
	}
	return
}
