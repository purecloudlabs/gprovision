// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package server

import (
	"context"
	"net"
	"time"

	"github.com/purecloudlabs/gprovision/pkg/log"
	"github.com/purecloudlabs/gprovision/pkg/oss/pblog/pb"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"
)

// Grpc entry point. lis and gsrv may be nil, in which case defaults are used.
func (a *allInOneSrvr) ServeGrpcWith(lis net.Listener, gsrv *grpc.Server) error {
	if a.store == nil {
		log.Fatalf("nil store")
	}
	if lis != nil {
		a.glis = lis
	}
	if gsrv == nil {
		gsrv = grpc.NewServer()
	}

	pb.RegisterLogServiceServer(gsrv, a)
	pb.RegisterRecordKeeperServer(gsrv, a)
	pb.RegisterSecretsServer(gsrv, a)
	pb.RegisterTimekeeperServer(gsrv, a)
	return gsrv.Serve(a.glis)
}

func pberr(err error) (*pb.GenericResponse, error) {
	if err != nil {
		return &pb.GenericResponse{EventType: pb.EvtType_ERROR}, err
	}
	return &pb.GenericResponse{EventType: pb.EvtType_SUCCESS}, nil
}

func ts(t time.Time) *pb.Timestamp {
	return &pb.Timestamp{TS: t.UnixNano()}
}
func tsStr(t *pb.Timestamp) string {
	if t == nil {
		return ""
	}
	return time.Unix(0, t.TS).Format(log.TimestampLayout)
}

//pb.LogServiceServer
func (a *allInOneSrvr) Log(ctx context.Context, evt *pb.LogEvent) (*pb.GenericResponse, error) {
	err := a.store.StoreLog(evt.SN, &pb.LogEvents{Evt: []*pb.LogEvent{evt}})
	return pberr(err)
}

//pb.RecordKeeperServer
func (a *allInOneSrvr) ReportCodename(ctx context.Context, name *pb.Codename) (*pb.GenericResponse, error) {
	now := time.Now()
	err := a.addLogEvent(&pb.LogEvent{
		SN:        name.SN,
		EventType: pb.EvtType_CODENAME,
		Payload:   name.Name,
		Time:      ts(now),
	})
	if err == nil {
		a.ah.Lock()
		defer a.ah.Unlock()
		e := a.ah.getent(name.SN)
		e.setPlat(name.Name, now)
	}
	return pberr(err)
}

func (a *allInOneSrvr) ReportState(ctx context.Context, s *pb.ProcessStage) (*pb.GenericResponse, error) {
	now := time.Now()
	err := a.addLogEvent(&pb.LogEvent{
		SN:        s.SN,
		EventType: pb.EvtType_STATE,
		Payload:   s.State.String(),
		Time:      ts(now),
	})
	if err == nil {
		a.ah.Lock()
		defer a.ah.Unlock()
		e := a.ah.getent(s.SN)
		e.setStage(s.State, now)
	}
	return pberr(err)
}

func (a *allInOneSrvr) StoreIPMIMACs(ctx context.Context, m *pb.MACs) (*pb.GenericResponse, error) {
	return pberr(a.store.StoreIpmiMacs(m.SN, *m))
}

func (a *allInOneSrvr) StoreMACs(ctx context.Context, m *pb.MACs) (*pb.GenericResponse, error) {
	return pberr(a.store.StoreMacs(m.SN, *m))
}

//pb.SecretsServer
var weakCreds = &pb.Credentials{
	OS:   "INSECURE11111111",
	BIOS: "WEAK1",
	IPMI: "I<3cipher0",
}

func (a *allInOneSrvr) GetCredentials(ctx context.Context, ident *pb.Identifier) (*pb.Credentials, error) {
	msg := "weak insecure credentials set"
	err := a.addLogEvent(&pb.LogEvent{SN: ident.Id, EventType: pb.EvtType_SETPW, Payload: msg})
	if err != nil {
		return nil, err
	}
	return weakCreds, nil
}

//pb.TimekeeperServer
func (a *allInOneSrvr) GetTime(ctx context.Context, _ *empty.Empty) (*pb.Timestamp, error) {
	return ts(time.Now()), nil
}
