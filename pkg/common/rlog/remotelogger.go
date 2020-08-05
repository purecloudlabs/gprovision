// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

//Interfaces related to setting up or mocking remote logging.
package rlog

import (
	"os"

	"github.com/purecloudlabs/gprovision/pkg/common"
	"github.com/purecloudlabs/gprovision/pkg/log"
)

//Sets up a remote logger on demand.
type RemoteLoggerSetuper interface {
	Setup(endpoint, id string) error
}

var rLoggerSetup RemoteLoggerSetuper

//sets the underlying RemoteLoggerSetuper impl for this package
func SetImpl(r RemoteLoggerSetuper) {
	if rLoggerSetup != nil {
		log.Log("rlog: overwrite non-nil impl")
	}
	rLoggerSetup = r
}

//Return true if RemoteLoggerSetuper impl is set
func HaveRLogSetup() bool { return rLoggerSetup != nil }

func Setup(endpoint, id string) error {
	if rLoggerSetup != nil {
		return rLoggerSetup.Setup(endpoint, id)
	}
	log.Log("rlog: impl unset")
	return os.ErrNotExist
}

// Sets up a mock remote log server; used in conjunction with a
// RemoteLoggerSetuper in tests (generally, but not exclusively,
// integration tests).
type RemoteLoggerMocker interface {
	//Sets up a mock remote logger.
	MockServer(f Fataler, tmpDir string) MockSrvr
	//Like MockServer, but you specify the port
	MockServerAt(f Fataler, tmpDir, port string) MockSrvr
}

var rLoggerMock RemoteLoggerMocker

//sets the underlying RemoteLoggerMocker impl for this package
func SetMockImpl(m RemoteLoggerMocker) {
	if rLoggerMock != nil {
		log.Log("rlog mock: overwriting previously-set impl")
	}
	rLoggerMock = m
}

//Return true if RemoteLoggerMocker impl is set
func HaveRLMock() bool { return rLoggerMock != nil }

func MockServer(f Fataler, tmpDir string) MockSrvr {
	if rLoggerMock != nil {
		return rLoggerMock.MockServer(f, tmpDir)
	}
	f.Fatal("rlog mock: impl unset")
	return nil
}

func MockServerAt(f Fataler, tmpDir, port string) MockSrvr {
	if rLoggerMock != nil {
		return rLoggerMock.MockServerAt(f, tmpDir, port)
	}
	f.Fatal("rlog mock: impl unset")
	return nil
}

//lets us avoid testing.T
type Fataler interface {
	Fatal(args ...interface{})
}

type MockSrvr interface {
	//frees resources
	Close()
	//returns port it listens on
	Port() int
	//return true if current state is that the given stage is finished
	CheckFinished(id, stage string) bool
	//returns all log entries as text, format is impl-defined
	Entries(id string) string
	//returns all ids that have been used while adding log entries, macs, ipmi macs
	Ids() []string
	//returns credentials mock server will hand out
	MockCreds(id string) common.Credentials
}
