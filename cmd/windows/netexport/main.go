// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

// Netexport.exe exports network config data from windows. See
// github.com/purecloudlabs/gprovision/pkg/netexport for additional details.
//
// This is cross-compiled by the CI job.
package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	fp "path/filepath"
	"sync"

	"github.com/purecloudlabs/gprovision/pkg/appliance/altIdent"
	"github.com/purecloudlabs/gprovision/pkg/common/strs"
	"github.com/purecloudlabs/gprovision/pkg/disktag"
	"github.com/purecloudlabs/gprovision/pkg/log"
	"github.com/purecloudlabs/gprovision/pkg/log/flags"
	nx "github.com/purecloudlabs/gprovision/pkg/netexport"
	netd "github.com/purecloudlabs/gprovision/pkg/systemd/networkd"
)

//in any binary with main.buildId string, it is set at compile time to $BUILD_INFO
var buildId string

var recoveryEnv = strs.EnvPrefix() + "RECOVERY"

//intel's SaveRestore.ps1 can read some data that isn't otherwise available, but ignores ipv6
//so, use two different methods to populate similar maps, then merge the maps

func main() {
	recovery := os.Getenv(recoveryEnv)
	logFile := fp.Join(recovery, strs.RecoveryLogDir(), "netexport.log")
	jsonFile := fp.Join(recovery, strs.RecoveryLogDir(), "netexport.json")
	ndTarball := fp.Join(recovery, strs.RecoveryLogDir(), "netd.tar")
	flag.StringVar(&jsonFile, "json", jsonFile, "path to json output file")
	flag.StringVar(&logFile, "log", logFile, "path to log file")
	flag.StringVar(&ndTarball, "netd", ndTarball, "path to systemd-networkd output tarball")
	flag.Parse()

	log.Logf("buildId: %s", buildId)

	if ndTarball == "" {
		log.Fatalf("path for tarball must not be empty")
	}

	if logFile != "" {
		dir := fp.Dir(logFile)
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			logFile = ""
			log.Logf("creating dir for %s: %s", logFile, err)
		} else {
			_, err := log.AddNamedFileLog(logFile)
			if err != nil {
				logFile = ""
				log.Logf("creating log: %s", err)
			}
		}
	}
	if logFile == "" {
		log.AddConsoleLog(flags.NA)
		log.FlushMemLog()
	}
	checkElevation()

	// Extract the platform from the disktag, write to a file
	// on recovery. Used in the event the dmi data is wrong.
	altIdent.Write(recovery, disktag.Platform(""))

	// interfaces, intelData: two slightly different map's. Intel's
	// representation is different enough that this is easiest, plus
	// it allows for parallelization
	interfaces := nx.NewIfMap()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := interfaces.GetAddrs()
		if err != nil {
			log.Logf("error in getAddrs: %s\n", err)
			log.SetFatalAction(log.FailAction{Terminator: func() { panic(err) }})
			log.Finalize()
		}
	}()

	intelData, err := nx.GetIntelData()
	if err != nil {
		panic(err)
	}
	wg.Wait()
	interfaces.Merge(intelData)

	if jsonFile != "" {
		err = os.MkdirAll(fp.Dir(jsonFile), 0755)
		if err != nil {
			panic(err)
		}
		var json []byte
		if err == nil {
			json, err = interfaces.ToJson()
		}
		if err == nil {
			//write to net.json, before and after linux conversion
			err = ioutil.WriteFile(jsonFile, json, 0644)
		}
		if err != nil {
			log.Logln("Failed to write json file:", err)
		}
	}
	err = os.MkdirAll(fp.Dir(ndTarball), 0755)
	if err == nil {
		err = netd.Export(interfaces, ndTarball)
	}
	if err != nil {
		log.Logf("error: %s\n", err)
		log.Finalize()
		os.Exit(1)
	}
	fmt.Println("success")
	log.Logln("success")
	os.Exit(0)
}
