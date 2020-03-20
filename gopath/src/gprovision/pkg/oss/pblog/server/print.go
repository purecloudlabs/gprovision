// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package server

import (
	"context"
	"fmt"
	"gprovision/pkg/common/rkeep"
	"gprovision/pkg/oss/pblog/pb"
	"io/ioutil"
	fp "path/filepath"
	"sync"
	"time"
)

func (a *allInOneSrvr) addLogEvent(le *pb.LogEvent) error {
	if a.store == nil {
		fmt.Printf("log with nil store: %s", le)
		return nil
	}
	return a.store.StoreLog(le.SN, &pb.LogEvents{Evt: []*pb.LogEvent{le}})
}

func (a *allInOneSrvr) StoreDocument(ctx context.Context, doc *pb.Document) (*pb.GenericResponse, error) {
	now := time.Now()
	if len(doc.Name) < len(doc.SN) || len(doc.Doctype) == 0 {
		err := fmt.Errorf("missing filename or doc type, got %s and %s", doc.Name, doc.Doctype)
		resp := &pb.GenericResponse{
			EventType: pb.EvtType_ERROR,
			ErrMsg:    err.Error(),
		}
		return resp, err
	}
	p := PrintableDoc{Document: doc}

	hold := (rkeep.PrintedDocType(doc.Doctype) == rkeep.PrintedDocQAV) && (QAHold > 0)
	if hold {
		HoldForPrinting <- p
	} else {
		err := p.print()
		if err != nil {
			err := fmt.Errorf("failed to print: %s", err)
			return &pb.GenericResponse{EventType: pb.EvtType_ERROR, ErrMsg: err.Error()}, err
		}
	}
	msg := fmt.Sprintf("received %d byte file %s for %s printing", len(doc.Body), doc.Name, doc.Doctype)
	if hold {
		msg += " (on hold for factory restore success; expires in " + QAHold.String() + ")"
	}
	le := &pb.LogEvent{
		SN:        doc.SN,
		EventType: pb.EvtType_PRINT,
		Time:      &pb.Timestamp{TS: now.UnixNano()},
		Payload:   msg,
	}
	err := a.addLogEvent(le)
	if err != nil {
		fmt.Println(err)
		return &pb.GenericResponse{EventType: pb.EvtType_ERROR, ErrMsg: err.Error()}, err
	}
	return &pb.GenericResponse{EventType: pb.EvtType_SUCCESS}, nil
}

type PrintableDoc struct {
	Expires time.Time
	*pb.Document
}

func (p *PrintableDoc) print() error {
	return ioutil.WriteFile(fp.Join(PrintDir, p.Name), p.Body, 0644)
}

type HeldDocs struct {
	a    *allInOneSrvr
	docs []*PrintableDoc
	mtx  sync.Mutex
	wg   *sync.WaitGroup
}

func (h *HeldDocs) patrol(done chan struct{}) {
	defer h.wg.Done()
	if QAHold == 0 {
		fmt.Println("QA Hold is 0 (indefinite). Docs will stick around in memory until service restart!")
		return
	}
loop:
	for {
		select {
		case <-done:
			break loop
		case <-time.After(QAHold / 2):
			h.patrolNow()
		}
	}
}
func (h *HeldDocs) patrolNow() {
	h.mtx.Lock()
	defer h.mtx.Unlock()
	now := time.Now()
	l := len(h.docs)
	for i := 0; i < l; {
		if h.docs[i].Expires.Before(now) {
			msg := fmt.Sprintf("document %s has expired - discarding", h.docs[i].Name)
			err := h.a.addLogEvent(&pb.LogEvent{
				SN:        h.docs[i].SN,
				Time:      &pb.Timestamp{TS: now.Unix()},
				EventType: pb.EvtType_PRINT_ERR,
				Payload:   msg,
			})
			if err != nil {
				fmt.Println(err)
			}
			h.docs = append(h.docs[:i], h.docs[i+1:]...)
			l--
			continue
		}
		i++
	}
}

func (h *HeldDocs) release(sn string) {
	h.mtx.Lock()
	defer h.mtx.Unlock()
	l := len(h.docs)
	for i := 0; i < l; {
		if h.docs[i].SN == sn {
			d := h.docs[i]
			err := d.print()
			if err != nil {
				fmt.Println("Releasing", d.Name, "for printing: error", err)
			}
			msg := fmt.Sprintf("Releasing %s for printing", d.Name)
			var msgType pb.EvtType = pb.EvtType_PRINT
			if err != nil {
				msg += fmt.Sprintf(": error %s", err)
				msgType = pb.EvtType_PRINT_ERR
			}
			now := time.Now().UnixNano()
			err = h.a.addLogEvent(&pb.LogEvent{
				SN:        d.SN,
				EventType: msgType,
				Time:      &pb.Timestamp{TS: now},
				Payload:   msg,
			})
			if err != nil {
				fmt.Println(err)
			}
			h.docs = append(h.docs[:i], h.docs[i+1:]...)
			l--
			continue
		}
		i++
	}
}
func (h *HeldDocs) add(doc *PrintableDoc) {
	doc.Expires = time.Now().Add(QAHold)
	h.mtx.Lock()
	defer h.mtx.Unlock()
	h.docs = append(h.docs, doc)
}

var ReleaseForPrinting chan string
var HoldForPrinting chan PrintableDoc

func (h *HeldDocs) manage(done chan struct{}) {
	defer h.wg.Done()
loop:
	for {
		select {
		case <-done:
			break loop
		case sn := <-ReleaseForPrinting:
			h.release(sn)
		case doc := <-HoldForPrinting:
			h.add(&doc)
		}
	}
}

// MonitorHolds starts background goroutines to process documents on hold. Set
// QAHold before calling.
func (a *allInOneSrvr) MonitorHolds(done chan struct{}) *sync.WaitGroup {
	if done == nil {
		done = make(chan struct{})
	}
	heldDocs := HeldDocs{a: a, wg: &sync.WaitGroup{}}
	heldDocs.wg.Add(2)
	ReleaseForPrinting = make(chan string)
	HoldForPrinting = make(chan PrintableDoc)
	go heldDocs.patrol(done)
	go heldDocs.manage(done)
	return heldDocs.wg
}
