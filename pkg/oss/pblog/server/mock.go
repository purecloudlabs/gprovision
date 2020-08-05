// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

// +build !release

package server

import (
	"fmt"
	"net"
	"os"
	fp "path/filepath"
	"sync"

	"github.com/purecloudlabs/gprovision/pkg/common"
	"github.com/purecloudlabs/gprovision/pkg/common/rlog"
	"github.com/purecloudlabs/gprovision/pkg/common/strs"
	"github.com/purecloudlabs/gprovision/pkg/oss/pblog/pb"
)

var mockOnce sync.Once

//mock impl, stays in memory
func UseMockImpl() { mockOnce.Do(func() { rlog.SetMockImpl(&mocker{}) }) }

type mocker struct{}

func (m *mocker) MockServer(f rlog.Fataler, tmpDir string) rlog.MockSrvr {
	return m.MockServerAt(f, tmpDir, ":0")
}

func (*mocker) MockServerAt(f rlog.Fataler, tmpdir, port string) rlog.MockSrvr {
	PrintDir = fp.Join(tmpdir, "print")
	err := os.MkdirAll(PrintDir, 0755)
	if err != nil {
		f.Fatal(err)
	}
	ms := &MockSrvr{
		allInOneSrvr: allInOneSrvr{
			store: newMockStore(),
		},
	}
	ms.starting.Add(1)
	go ms.ServeAt(port)
	ms.starting.Wait()
	return ms
}

type MockSrvr struct {
	allInOneSrvr
}

func (ms *MockSrvr) CheckFinished(sn, stage string) bool {
	var ps pb.ProcessState
	switch stage {
	case strs.MfgLogPfx():
		ps = pb.ProcessState_MFG_SUCCEEDED
	case strs.FRLogPfx():
		ps = pb.ProcessState_FR_SUCCEEDED
	default:
		fmt.Printf("unknown stage %s\n", stage)
		return false
	}

	ms.ah.Lock()
	defer ms.ah.Unlock()
	for _, e := range ms.ah.entries {
		if e.sn == sn {
			return e.state == ps
		}
	}
	return false
}

func (ms *MockSrvr) Port() int {
	return ms.lis.Addr().(*net.TCPAddr).Port
}

func (ms *MockSrvr) Ids() []string { return ms.allInOneSrvr.store.Ids() }

func (ms *MockSrvr) Entries(id string) string {
	entries, _ := ms.allInOneSrvr.store.RetrieveLog(id)
	var s string
	for _, le := range entries.Evt {
		s += fmt.Sprintf("%10s %s [%10s] %s\n", le.SN, tsStr(le.Time), le.EventType.String(), le.Payload)
	}
	return s
}

func (ms *MockSrvr) MockCreds(sn string) common.Credentials {
	return common.Credentials{
		OS:   weakCreds.OS,
		BIOS: weakCreds.BIOS,
		IPMI: weakCreds.IPMI,
	}
}

//mockStore: an in-memory store for mocking
type mockStore struct {
	sync.Mutex
	macs, imacs map[string]pb.MACs
	logs        map[string]pb.LogEvents
	//pass        map[string]pb.Credentials //no need to store, constant
}

func newMockStore() *mockStore {
	return &mockStore{
		macs:  make(map[string]pb.MACs),
		imacs: make(map[string]pb.MACs),
		logs:  make(map[string]pb.LogEvents),
	}
}

var _ Persister = (*mockStore)(nil)

//return all ids that have been used when logging or reporting macs/ipmi macs
func (ms *mockStore) Ids() []string {
	//use a map to deduplicate
	ids := make(map[string]interface{})
	ms.Lock()
	defer ms.Unlock()
	for k := range ms.macs {
		ids[k] = nil
	}
	for k := range ms.imacs {
		ids[k] = nil
	}
	for k := range ms.logs {
		ids[k] = nil
	}
	var idlist []string
	for k := range ids {
		idlist = append(idlist, k)
	}
	return idlist
}

func (ms *mockStore) Close() error {
	ms.Lock()
	defer ms.Unlock()
	ms.macs = nil
	ms.imacs = nil
	ms.logs = nil
	return nil
}
func (ms *mockStore) RetrieveLog(id string) (pb.LogEvents, error) {
	ms.Lock()
	defer ms.Unlock()
	return ms.logs[id], nil
}
func (ms *mockStore) StoreLog(id string, le *pb.LogEvents) error {
	ms.Lock()
	defer ms.Unlock()
	l := ms.logs[id]
	l.Evt = append(l.Evt, le.Evt...)
	ms.logs[id] = l
	return nil
}
func (ms *mockStore) RetrieveMacs(id string) (m pb.MACs, err error) {
	ms.Lock()
	defer ms.Unlock()
	return ms.macs[id], nil
}
func (ms *mockStore) StoreMacs(id string, m pb.MACs) error {
	ms.Lock()
	defer ms.Unlock()
	ms.macs[id] = m
	return nil
}
func (ms *mockStore) RetrieveIpmiMacs(id string) (pb.MACs, error) {
	ms.Lock()
	defer ms.Unlock()
	return ms.imacs[id], nil
}
func (ms *mockStore) StoreIpmiMacs(id string, m pb.MACs) error {
	ms.Lock()
	defer ms.Unlock()
	ms.imacs[id] = m
	return nil
}
func (ms *mockStore) RetrievePass(id string) (p *pb.Credentials, err error) {
	//not stored in mock
	return nil, nil
}
func (ms *mockStore) StorePass(id string, p *pb.Credentials) error {
	//not stored in mock
	return nil
}
