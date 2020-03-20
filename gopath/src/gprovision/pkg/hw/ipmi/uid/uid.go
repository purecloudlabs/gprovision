// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

//Package uid allows control of the IPMI UID light (AKA Chassis ID light).
package uid

import (
	"fmt"
	"os"
	"os/exec"
	"time"
)

/*
ipmiutil alarms (also ipmiutil leds)
   -iN    Sets the Chassis Identify feature, which can be an LED  or  some
	      other  alarm.   If  N=0, turn off the Chassis ID, otherwise turn
	      the ID on for N seconds.	N=255 will  turn  on  the  ID  indefi-
	      nitely, if it is IPMI 2.0.
*/

//Turns UID led on for given amount of time (255=indefinitely). Returns immediately.
func Once(seconds uint8) error {
	return exec.Command("ipmiutil", "leds", fmt.Sprintf("-i%d", seconds)).Run()
}

func Off() error { return Once(0) }

func On() error { return Once(255) }

//Blink UID light until channel is closed.
func BlinkUntil(done chan struct{}, onTime uint8) error {
	if onTime <= 1 {
		return os.ErrInvalid
	}
	period := time.Duration(onTime) * time.Second * 2
	for {
		err := Once(onTime)
		if err != nil {
			return err
		}
		select {
		case <-done:
			return Off()
		case <-time.After(period):
		}
	}
}
