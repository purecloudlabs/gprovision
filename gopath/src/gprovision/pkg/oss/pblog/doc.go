// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

// Package pblog is the client for a logger using gRPC. Additional non-logging
// functionality needed for provisioning is included. Including the additional
// functionality in this package rather than another is merely for convenience -
// in production implementations, it may be preferred to separate them.
//
// The protocol buffer/grpc definitions are in the pb sub-package, and the
// server impl is in the server sub-package.
package pblog
