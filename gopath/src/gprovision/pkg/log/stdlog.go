// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package log

import (
	"gprovision/pkg/log/flags"
	"log"
)

// AdaptStdlog redirects output from the system pkg "log" to this logger.
//
// If resetSLFlags is true, the system log's flags are cleaned up so that time
// info isn't added to the entry twice. This happens immediately and on first
// use, the assumption being the flags may be set/initialized later on, and that
// one entry with extra data is better than many.
//
// Use nil for logger if the logger in question is the predefined "standard" one.
func AdaptStdlog(logger *log.Logger, level flags.Flag, resetSLFlags bool) {
	sa := &stdAdapter{
		level:        level,
		resetSLFlags: resetSLFlags,
		logger:       logger,
	}
	if resetSLFlags {
		sa.resetSlFlags()
	}
	if logger == nil {
		log.SetOutput(sa)
	} else {
		logger.SetOutput(sa)
	}
}

type stdAdapter struct {
	resetSLFlags bool
	level        flags.Flag
	logger       *log.Logger
}

func (sa *stdAdapter) Write(b []byte) (int, error) {
	if sa.resetSLFlags {
		sa.resetSLFlags = false //once only
		go sa.resetSlFlags()
	}
	FlaggedLogf(sa.level, string(b))
	return len(b), nil
}

//clear time-related flags on std log
//NOTE log's internal state is guarded by a mutex. Exercise caution in calling synchronously.
func (sa *stdAdapter) resetSlFlags() {
	if sa.logger == nil {
		log.SetFlags(log.Flags() &^ (log.Ldate | log.Ltime | log.Lmicroseconds))
	} else {
		sa.logger.SetFlags(sa.logger.Flags() &^ (log.Ldate | log.Ltime | log.Lmicroseconds))
	}
}
