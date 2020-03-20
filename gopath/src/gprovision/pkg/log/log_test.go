// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package log_test

import (
	"fmt"
	"gprovision/pkg/log"
	"gprovision/pkg/log/flags"
	"gprovision/pkg/log/lcd"
	"io/ioutil"
	"os"
	"sync"
)

func Example() {
	log.AddConsoleLog(flags.NA) // NA -> everything will display on console
	//Note - must call cfa.Find() or equivalent before AddLcdLog
	_ = lcd.AddLcdLog(flags.EndUser) // EndUser -> log.Msgf() will display
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		log.Log("Concurrency is safe but order is not guaranteed.")
		log.Log("Ensure goroutines have exited before log.Finalize() -")
		log.Log("_particularly_ in tests")
		wg.Done()
	}()
	log.Msg("this will show up on the console and lcd")
	log.Log("this will show up on the console but not lcd")

	//Required for AddFileLog(), some others. Here, becomes filename prefix.
	log.SetPrefix("testlog")

	// Add a fileLog. It will contain above events because it first reads
	// existing events.
	filename, err := log.AddFileLog("/tmp")
	if err != nil {
		log.Fatalf("creating file log: %s", err)
	}
	if false {
		fmt.Println("logging to", filename)
	}
	log.Msgf("%d more events", 999)
	wg.Wait() //ensure goroutine finishes
	log.Finalize()
	f, _ := ioutil.ReadFile(filename)
	fmt.Printf("log contents\n............\n%s", string(f))

	//cleanup
	os.Remove(filename)

	/* output will be something like
	log contents
	............
	-- 20191021_1016 -- this will show up on the console and lcd
	*- 20191021_1016 *- this will show up on the console but not lcd
	...
	*/
}
