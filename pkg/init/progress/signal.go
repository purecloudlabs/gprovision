// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package progress

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/purecloudlabs/gprovision/pkg/hw/kmsg"
)

type sigChan chan os.Signal

func setupSignal() sigChan {
	sig := make(sigChan, 1)
	signal.Notify(sig, syscall.SIGUSR1)
	return sig
}

func waitForSignal(km *kmsg.KmsgWithPrio, sig sigChan, timeout time.Duration) {
	select {
	case <-time.After(timeout):
		km.Printf("switch_root taking too long, exiting")
		os.Exit(1)
	case <-sig:
		km.Printf("got signal")
		return
	}
}
