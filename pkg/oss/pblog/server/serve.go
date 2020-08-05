// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package server

import (
	"flag"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/purecloudlabs/gprovision/pkg/log"

	"github.com/soheilhy/cmux"
	"golang.org/x/sync/errgroup"
)

var (
	Port     = ":8080"
	QAHold   time.Duration
	PrintDir string
)

var flagOnce sync.Once

func Flags() {
	flagOnce.Do(func() {
		flag.StringVar(&Port, "port", Port, "override port")
		flag.StringVar(&PrintDir, "printDir", ".", "dir to write printable docs")
		flag.DurationVar(&QAHold, "qaMaxHold", 20*time.Minute,
			"factory restore must complete within this time or qa doc will be discarded (use '0s' to disable discard)")
		flag.Parse()
	})
}

type allInOneSrvr struct {
	store           Persister
	lis, glis, hlis net.Listener
	ah              activityHistory
	starting        sync.WaitGroup
}

func (a *allInOneSrvr) Serve() {
	a.ServeAt(Port)
}
func (a *allInOneSrvr) ServeAt(port string) {
	if a.store == nil {
		log.Fatalf("store is nil")
	}
	var err error
	a.lis, err = net.Listen("tcp", port)
	if err != nil {
		log.Fatalf(err.Error())
	}

	//TODO TLS support

	m := cmux.New(a.lis)
	// Note - the cmux example is outdated. see this issue (comment is for tls):
	// https://github.com/soheilhy/cmux/issues/64#issuecomment-494565308
	// and workaround documented for use with java clients,
	//  https://github.com/soheilhy/cmux#limitations
	a.glis = m.MatchWithWriters(cmux.HTTP2MatchHeaderFieldSendSettings("content-type", "application/grpc"))
	a.hlis = m.Match(cmux.HTTP1Fast())

	g := new(errgroup.Group)
	g.Go(func() error { return a.ServeGrpcWith(nil, nil) })
	g.Go(func() error { return a.ServeHttpWith(nil, nil) })
	g.Go(func() error { return m.Serve() })

	a.starting.Done()

	err = g.Wait()
	// see ErrNetClosing in $GOROOT/src/internal/poll/fd.go and comments around
	// the string in $GOROOT/src/net/error_test.go - this string will not change
	closeStr := "use of closed network connection"
	if err != nil && strings.Contains(err.Error(), closeStr) {
		err = nil
	}
	if err != nil {
		log.Logf("run server: %s", err)
	}
}

func NewServer(dbfile string) *allInOneSrvr {
	a := &allInOneSrvr{store: OpenDB(dbfile)}
	a.starting.Add(1)
	return a
}

func (aio *allInOneSrvr) Close() {
	log.Log("shutting down server...")
	if aio != nil {
		if aio.glis != nil {
			aio.glis.Close()
		}
		if aio.hlis != nil {
			aio.hlis.Close()
		}
		if aio.lis != nil {
			aio.lis.Close()
		}
		if aio.store != nil {
			aio.store.Close()
		}
	}
}
