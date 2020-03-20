// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package pblog

import (
	"context"
	"fmt"
	"gprovision/pkg/common"
	"gprovision/pkg/common/rkeep"
	"gprovision/pkg/common/rlog"
	"gprovision/pkg/common/strs"
	"gprovision/pkg/log"
	"gprovision/pkg/log/flags"
	"gprovision/pkg/oss/pblog/pb"
	"os"
	"sync"
	"time"

	empty "github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"
)

const LogIdent = "PBLog"

var rlOnce sync.Once

func UseRLoggerSetup() { rlOnce.Do(func() { rlog.SetImpl(&RLogSetup{}) }) }

type RLogSetup struct{}

func (*RLogSetup) Setup(endpoint, id string) error {
	_, err := AddPBLog(endpoint, id, 0)
	return err
}

// If UseRKeeper is called before pblog is set up by AddPBLog, can't actually
// set up as a recordkeeper because the same Pbl is needed. Workaround by
// setting this bool, which will cause recordkeeper to be set up when AddPBLog
// is called.
var initRKeep bool

func AddPBLog(endpoint, sn string, flags flags.Flag) (AllInOne, error) {
	ctx := context.Background()
	dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(dialCtx, endpoint,
		grpc.WithInsecure(),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, err
	}

	p := &Pbl{
		flags: flags,
		ctx:   ctx,
		conn:  conn,
		sn:    sn,
		lc:    pb.NewLogServiceClient(conn),
		rc:    pb.NewRecordKeeperClient(conn),
		tc:    pb.NewTimekeeperClient(conn),
		sc:    pb.NewSecretsClient(conn),
	}
	if initRKeep {
		rkeep.SetImpl(p)
	}
	return p, log.AddLogger(p, true)
}

type Pbl struct {
	u        common.Unit
	sn       string
	notfirst bool
	flags    flags.Flag
	next     log.StackableLogger
	ctx      context.Context
	conn     *grpc.ClientConn
	lc       pb.LogServiceClient
	rc       pb.RecordKeeperClient
	tc       pb.TimekeeperClient
	sc       pb.SecretsClient
}

// Must be callable even when conn is not open, as something may try to log
// after Finalize() at shutdown.
func (p *Pbl) AddEntry(e log.LogEntry) {
	if p.conn != nil && p.lc != nil && e.Flags&flags.NotWire == 0 {
		if p.flags == 0 || e.Flags&p.flags > 0 {
			if !p.notfirst {
				p.notfirst = true
				p.ReportState("start")
			}
			p.addEntry(e)
		}
	} else {
		if p.lc == nil {
			fmt.Fprintln(os.Stderr, "pblog: nil lc")
		}
		if p.conn == nil {
			fmt.Fprintln(os.Stderr, "pblog: nil conn")
		}
	}
	if p.next != nil {
		p.next.AddEntry(e)
	}
}
func (p *Pbl) addEntry(e log.LogEntry) {
	in := &pb.LogEvent{
		SN:   p.sn,
		Time: &pb.Timestamp{TS: e.Time.UnixNano()},
	}
	if len(e.Args) > 0 {
		in.Payload = fmt.Sprintf(e.Msg, e.Args...)
	} else {
		in.Payload = e.Msg
	}
	switch {
	case p.flags&flags.Fatal > 0:
		in.EventType = pb.EvtType_ERROR
	case p.flags&flags.EndUser > 0:
		in.EventType = pb.EvtType_MSG
	default:
		in.EventType = pb.EvtType_LOG
	}
	_, err := p.lc.Log(p.ctx, in)
	if err != nil {
		log.FlaggedLogf(flags.NotWire, "error logging: %s", err)
	}
}

func (p *Pbl) Finalize() {
	if p.conn != nil {
		err := p.conn.Close()
		if err != nil {
			fmt.Fprintf(os.Stderr, "pblog conn close: err %s", err)
		}
		p.conn = nil
	}
	if p.next != nil {
		p.next.Finalize()
	}
}

func (p *Pbl) ForwardTo(sl log.StackableLogger) {
	if p.next == nil || sl == nil {
		p.next = sl
	} else {
		panic("next already set")
	}
}

func (p *Pbl) Ident() string             { return LogIdent }
func (p *Pbl) Next() log.StackableLogger { return p.next }

var rkOnce sync.Once

func UseRKeeper() {
	rkOnce.Do(func() {
		pbl := log.FindInStack(LogIdent)
		if pbl != nil {
			rkeep.SetImpl(pbl.(*Pbl))
		} else {
			initRKeep = true
		}
	})
}

type AllInOne interface {
	log.StackableLogger
	rkeep.RecordKeeper
	common.Credentialer
}

func (p *Pbl) SetUnit(u common.Unit) {
	p.u = u
}

func (p *Pbl) GetTime() string {
	log.Logf("getting time")
	if p.tc == nil {
		log.FlaggedLogf(flags.NotWire, "pblog: nil tc")
		return ""
	}
	if p.conn == nil {
		log.FlaggedLogf(flags.NotWire, "pblog: nil conn")
		return ""
	}
	resp, err := p.tc.GetTime(p.ctx, &empty.Empty{})
	if err != nil {
		//in tests, handleGrpcErr returns - in normal circumstances, it does not
		p.handleGrpcErr(nil, err)
		return ""
	}
	t := time.Unix(0, resp.TS)
	return t.Format("2006-01-02 15:04:05")
}

func (p *Pbl) ReportCodename(c string) {
	if p.rc == nil {
		log.FlaggedLogf(flags.NotWire, "pblog: nil rc")
		return
	}
	if p.conn == nil {
		log.FlaggedLogf(flags.NotWire, "pblog: nil conn")
		return
	}
	resp, err := p.rc.ReportCodename(p.ctx, &pb.Codename{SN: p.sn, Name: c})
	p.handleGrpcErr(resp, err)
}

func pstate(state string) pb.ProcessState {
	switch log.GetPrefix() {
	case strs.MfgLogPfx():
		switch state {
		case "start":
			return pb.ProcessState_MFG_STARTED
		case "fail":
			return pb.ProcessState_MFG_FAILED
		case "finish":
			return pb.ProcessState_MFG_SUCCEEDED
		}
	case strs.FRLogPfx():
		switch state {
		case "start":
			return pb.ProcessState_FR_STARTED
		case "fail":
			return pb.ProcessState_FR_FAILED
		case "finish":
			return pb.ProcessState_FR_SUCCEEDED
		}
	case "init":
		switch state {
		case "start":
			return pb.ProcessState_INIT_STARTED
		case "fail":
			return pb.ProcessState_INIT_FAILED
		case "finish":
			return pb.ProcessState_INIT_SUCCEEDED
		}
	}
	return pb.ProcessState_UNKNOWN
}

func (p *Pbl) ReportState(state string) {
	if p.rc == nil {
		log.FlaggedLogf(flags.NotWire, "pblog: nil rc")
		return
	}
	if p.conn == nil {
		log.FlaggedLogf(flags.NotWire, "pblog: nil conn")
		return
	}
	p.handleGrpcErr(p.rc.ReportState(p.ctx, &pb.ProcessStage{
		SN:    p.sn,
		State: pstate(state),
	}))
}

func (p *Pbl) ReportFailure(f string) {
	if p.lc == nil {
		log.FlaggedLogf(flags.NotWire, "pblog: nil lc")
		return
	}
	if p.conn == nil {
		log.FlaggedLogf(flags.NotWire, "pblog: nil conn")
		return
	}
	p.handleGrpcErr(p.lc.Log(p.ctx, &pb.LogEvent{
		SN:        p.sn,
		Time:      &pb.Timestamp{TS: time.Now().UnixNano()},
		Payload:   f,
		EventType: pb.EvtType_ERROR,
	}))
	p.ReportState("fail")
}
func (p *Pbl) ReportFinished(f string) {
	if p.lc == nil {
		log.FlaggedLogf(flags.NotWire, "pblog: nil lc")
		return
	}
	if p.conn == nil {
		log.FlaggedLogf(flags.NotWire, "pblog: nil conn")
		return
	}
	p.handleGrpcErr(p.lc.Log(p.ctx, &pb.LogEvent{
		SN:        p.sn,
		Time:      &pb.Timestamp{TS: time.Now().UnixNano()},
		Payload:   f,
		EventType: pb.EvtType_MSG,
	}))
	p.ReportState("finish")
}

func (p *Pbl) StoreIPMIMACs(im []string) {
	if p.rc == nil {
		log.FlaggedLogf(flags.NotWire, "pblog: nil rc")
		return
	}
	if p.conn == nil {
		log.FlaggedLogf(flags.NotWire, "pblog: nil conn")
		return
	}
	resp, err := p.rc.StoreIPMIMACs(p.ctx, &pb.MACs{SN: p.sn, MAC: im})
	p.handleGrpcErr(resp, err)
}

func (p *Pbl) StoreMACs(m []string) {
	if p.rc == nil {
		log.FlaggedLogf(flags.NotWire, "pblog: nil rc")
		return
	}
	if p.conn == nil {
		log.FlaggedLogf(flags.NotWire, "pblog: nil conn")
		return
	}
	resp, err := p.rc.StoreMACs(p.ctx, &pb.MACs{SN: p.sn, MAC: m})
	p.handleGrpcErr(resp, err)
}
func (p *Pbl) StoreDocument(name string, doctype rkeep.PrintedDocType, body []byte) {
	if p.rc == nil {
		log.FlaggedLogf(flags.NotWire, "pblog: nil rc")
		return
	}
	if p.conn == nil {
		log.FlaggedLogf(flags.NotWire, "pblog: nil conn")
		return
	}
	resp, err := p.rc.StoreDocument(p.ctx, &pb.Document{
		SN:      p.sn,
		Name:    name,
		Doctype: string(doctype),
		Body:    body,
	})
	p.handleGrpcErr(resp, err)
}

//GetCredentials is part of imaging.Credentialer interface.
func (p *Pbl) GetCredentials(ident string) common.Credentials {
	if p.sc == nil {
		log.FlaggedLogf(flags.NotWire, "pblog: nil sc")
		return common.Credentials{}
	}
	if p.conn == nil {
		log.FlaggedLogf(flags.NotWire, "pblog: nil conn")
		return common.Credentials{}
	}
	resp, err := p.sc.GetCredentials(p.ctx, &pb.Identifier{Id: ident})
	p.handleGrpcErr(nil, err)
	return common.Credentials{
		OS:   resp.OS,
		BIOS: resp.BIOS,
		IPMI: resp.IPMI,
	}
}

//SetEP is part of imaging.Credentialer interface. no-op.
func (*Pbl) SetEP(string) {}

func (p *Pbl) handleGrpcErr(resp *pb.GenericResponse, err error) {
	if err != nil {
		log.FlaggedLogf(flags.NotWire|flags.Fatal, "grpc: error %s, resp %#v", err, resp)
	}
}
