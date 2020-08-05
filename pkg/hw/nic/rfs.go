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
	fp "path/filepath"

	"github.com/purecloudlabs/gprovision/pkg/log"
)

const (
	rfsEntries     = 131072
	rfsEntriesFile = "/proc/sys/net/core/rps_sock_flow_entries"
)

//Receive Flow Scaling - set total number of RFS entries, as well as number per RSS queue
func (nic Nic) RfsConfig() {
	err := ioutil.WriteFile(rfsEntriesFile, []byte(fmt.Sprintf("%d\n", rfsEntries)), 0644)
	if err != nil {
		log.Logf("error writing to rfs entries file: %s\n", err)
	}
	queues := nic.Queues("rx-")
	count := []byte(fmt.Sprintf("%d\n", rfsEntries/len(queues)))
	for _, q := range queues {
		err = ioutil.WriteFile(fp.Join(q, "rps_flow_cnt"), count, 0644)
		if err != nil {
			log.Logf("error writing to rps flow count file: %s\n", err)
		}
	}
}
