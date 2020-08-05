// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

//Test app for github.com/purecloudlabs/gprovision/pkg/hw/ipmi/uid, blinking the IPMI UID light.
package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/purecloudlabs/gprovision/pkg/hw/ipmi/uid"
)

func main() {
	onTime := flag.Uint("on", 5, "on/off time for uid, seconds")
	total := flag.Duration("until", time.Minute, "stop blinking after this much time has passed")
	flag.Parse()
	done := make(chan struct{})
	go func() {
		err := uid.BlinkUntil(done, uint8(*onTime))
		if err != nil {
			fmt.Printf("BeepUntil error: %s\n", err)
		}
	}()
	time.Sleep(*total)
	close(done)
	/* pausing here is not strictly necessary, but exiting immediately after close()
	   would mean the goroutine didn't get to do any cleanup if there was any.
	*/
	time.Sleep(time.Second / 10)
}
