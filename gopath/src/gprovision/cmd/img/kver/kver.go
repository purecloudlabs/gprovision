// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package main

import (
	"encoding/json"
	"fmt"
	"gprovision/pkg/fileutil/kver"
	"log"
	"os"
)

var buildId string

func main() {
	log.SetFlags(0)
	log.Printf("buildId: %s", buildId)

	if len(os.Args) != 2 {
		log.Println("use:", os.Args[0], "/path/to/kernel")
		log.Println("     prints kernel build info")
		log.Fatalf("need exactly one arg")
	}
	k, err := os.Open(os.Args[1])
	if err != nil {
		log.Fatalf("%s", err)
	}
	defer k.Close()
	desc, err := kver.GetKDesc(k)
	if err != nil {
		log.Fatalf("%s", err)
	}
	info, err := kver.ParseDesc(desc)
	if err != nil {
		log.Fatalf("%s", err)
	}
	j, err := json.MarshalIndent(info, "", "    ")
	if err != nil {
		log.Fatalf("%s", err)
	}
	//on stdout
	fmt.Printf("%s", string(j))
}
