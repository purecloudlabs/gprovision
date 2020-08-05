// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

// Subpackages contain code for use in provisioning (also referred to as
// manufacture) and factory restore (aka recovery) as well as services which
// run in the image.
//
// Three main flavors of kernel can be built. The kernel code remains the same,
// with the initramfs changing. The flavors are:
//
//    - provisioning: pxebooted, used to image large numbers of servers with
//      full functionality. Requires at least two physical storage devices
//      within the unit - a small recovery device containing a copy of the
//      image, and a (generally much larger) primary device onto which the image
//      is decompressed. Recovery device could be a usb key, ssd, etc. Depends
//      upon additional infrastructure - dhcp and tftp config for pxe, ipxe and
//      boot menu(s), http file server, logging server, and some mechanism to
//      store (and optionally, generate) passwords.
//
//    - recipe: a "downloadable installer"
//       - downloaded by a customer for installation onto hardware they source,
//         for use in regions where shipping from the US is painful.
//       - combines the QA functionality of provisioning with some functionality
//         of factory restore. Imaged unit _lacks_ factory restore functionality
//         on its own as that requires the secondary storage.
//
//    - norm: after recipe or provisioning, this is the kernel that is always
//      used. Includes factory restore functionality, though this only functions
//      if the unit was provisioned (as opposed to recipe). For provisioned
//      units, factory restore:
//        - will be triggered if the primary device's filesystem (or a flag file
//          therein) is missing, or
//        - may be triggered on demand by the user.
//      In all other cases, the root volume is located and mounted, the
//      initramfs is cleaned up, and systemd is started.
//
// Use `mage` to build these targets.
//
package gprovision
