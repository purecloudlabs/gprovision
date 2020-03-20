// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package qa

import (
	"bufio"
	"fmt"
	"gprovision/pkg/log"
	"io/ioutil"
	"os"
	fp "path/filepath"
	"strconv"
	"strings"
)

type CPUInfo struct {
	Model string //use one string, and ensure that all cpu's match that model
	//better to use family/model like winQA? "Intel64 Family 6 Model 79"
	Cores   int
	Sockets int  //how to differentiate between sockets and cores??
	Errors  bool `json:"-"`
}

func (c CPUInfo) String() string {
	return fmt.Sprintf(`Model="%s" Cores=%d Sockets=%d`, c.Model, c.Cores, c.Sockets)
}

func (c *CPUInfo) Read() {
	index := 0
	ci, err := os.Open("/proc/cpuinfo")
	if err != nil {
		log.Logf("CPUInfo: %s", err)
		c.Errors = true
	}
	defer ci.Close()

	scanner := bufio.NewScanner(ci)
	for scanner.Scan() {
		l := scanner.Text()
		fields := strings.SplitN(l, ":", 2)
		if len(fields) != 2 {
			continue
		}
		key := strings.TrimSpace(fields[0])
		value := strings.TrimSpace(fields[1])
		switch key {
		case "processor":
			var err error
			index, err = strconv.Atoi(value)
			if err != nil {
				log.Logf("CPUInfo: %s", err)
				c.Errors = true
			}
			c.Cores += 1
		case "model name":
			if c.Model == "" && index == 0 {
				c.Model = value
			} else {
				if c.Model != value {
					log.Logf("CPU model anomaly: cpu[0]=%s, cpu[%d]=%s", c.Model, index, value)
					c.Errors = true
				}
			}
		/*case "core id":  */ //doesn't work for counting sockets
		default:
			//ignore everything else
		}
	}
	if err := scanner.Err(); err != nil {
		log.Logf("CPUInfo: %s", err)
		c.Errors = true
	}
	sockets := make(map[int]int)
	sysCpuDir := "/sys/devices/system/cpu"
	entries, err := ioutil.ReadDir(sysCpuDir)
	if err != nil {
		log.Logf("CPUInfo: %s", err)
		c.Errors = true
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "cpu") && e.IsDir() {
			f, err := ioutil.ReadFile(fp.Join(sysCpuDir, e.Name(), "topology/physical_package_id"))
			var s int
			if err != nil && os.IsNotExist(err) {
				//treat as socket 0
				s = 0
			} else if err != nil {
				log.Logf("CPUInfo: %s", err)
				c.Errors = true
			} else {
				s, err = strconv.Atoi(strings.TrimSpace(string(f)))
				if err != nil {
					log.Logf("CPUInfo: %s", err)
					c.Errors = true
				}
			}
			sockets[s] += 1
		}
	}
	c.Sockets = len(sockets)
}

func ReadInfo() *CPUInfo {
	c := &CPUInfo{}
	c.Read()
	return c
}
