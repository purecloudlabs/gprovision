// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package server

import (
	"net"
	"net/http"

	"github.com/purecloudlabs/gprovision/pkg/log"

	"github.com/bmizerany/pat"
)

// Http entry point. When used in integ tests, pass non-nil lis and/or srvr to
// specify port to be used and to allow graceful shutdown, respectively.
func (a *allInOneSrvr) ServeHttpWith(lis net.Listener, srvr *http.Server) error {
	if a.store == nil {
		log.Fatalf("nil store")
	}
	if lis != nil {
		a.hlis = lis
	}
	if srvr == nil {
		srvr = &http.Server{}
	}

	mux := pat.New()
	mux.Get("/view/:sn/", http.HandlerFunc(a.view)) //web server
	mux.Get("/recent/", http.HandlerFunc(a.recentHist))
	mux.Get("/unit-state/:sn", http.HandlerFunc(a.unitState))
	mux.Get("/bg", http.HandlerFunc(bg))
	mux.Get("/css", http.HandlerFunc(css))
	mux.Get("/", http.RedirectHandler("/recent/", http.StatusMovedPermanently))

	srvr.Handler = mux
	return srvr.Serve(a.hlis)
}
