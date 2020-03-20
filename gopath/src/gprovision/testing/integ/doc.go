// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

//Package integ contains integ tests run by mage. Certain env vars must
//be set, or the tests will be skipped.
//
// Required:
// * INFRA_ROOT
// * UROOT_KERNEL
// * UROOT_QEMU
//
// Optional:
// * TEMPDIR
//
// See mage for what values are used.
package integ
