// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package strs

func IntegEnv() string           { return EnvPrefix() + "INTEG_TEST" }
func VerboseEnv() string         { return EnvPrefix() + "VERBOSE" }
func LogEnv() string             { return EnvPrefix() + "XLOG" }
func ContinueLoggingEnv() string { return EnvPrefix() + "CONT_LOGGING" }
func CoreEnv() string            { return EnvPrefix() + "TRACE_ROOT" }
func MfgTestEnv() string         { return EnvPrefix() + "MFG_TEST" }

//historically, these two lacked a company/product-specific prefix

func EraseEnv() string    { return "DATA_ERASE" }
func NoRebootEnv() string { return "NO_REBOOT" }
