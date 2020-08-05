// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package ipmi

import (
	"testing"
)

//ipmitool channel getaccess 1
var chGetAccess1 = `Maximum User IDs     : 10
Enabled User IDs     : 2

User ID              : 1
User Name            :
Fixed Name           : Yes
Access Available     : call-in / callback
Link Authentication  : disabled
IPMI Messaging       : disabled
Privilege Level      : Unknown (0x00)
Enable Status        : enabled

User ID              : 2
User Name            : ADMIN
Fixed Name           : Yes
Access Available     : callback
Link Authentication  : disabled
IPMI Messaging       : enabled
Privilege Level      : ADMINISTRATOR
Enable Status        : enabled

User ID              : 3
User Name            :
Fixed Name           : No
Access Available     : call-in / callback
Link Authentication  : disabled
IPMI Messaging       : disabled
Privilege Level      : Unknown (0x00)
Enable Status        : disabled

User ID              : 4
User Name            :
Fixed Name           : No
Access Available     : call-in / callback
Link Authentication  : disabled
IPMI Messaging       : disabled
Privilege Level      : Unknown (0x00)
Enable Status        : disabled

User ID              : 5
User Name            :
Fixed Name           : No
Access Available     : call-in / callback
Link Authentication  : disabled
IPMI Messaging       : disabled
Privilege Level      : Unknown (0x00)
Enable Status        : disabled

User ID              : 6
User Name            :
Fixed Name           : No
Access Available     : call-in / callback
Link Authentication  : disabled
IPMI Messaging       : disabled
Privilege Level      : Unknown (0x00)
Enable Status        : disabled

User ID              : 7
User Name            :
Fixed Name           : No
Access Available     : call-in / callback
Link Authentication  : disabled
IPMI Messaging       : disabled
Privilege Level      : Unknown (0x00)
Enable Status        : disabled

User ID              : 8
User Name            :
Fixed Name           : No
Access Available     : call-in / callback
Link Authentication  : disabled
IPMI Messaging       : disabled
Privilege Level      : Unknown (0x00)
Enable Status        : disabled

User ID              : 9
User Name            :
Fixed Name           : No
Access Available     : call-in / callback
Link Authentication  : disabled
IPMI Messaging       : disabled
Privilege Level      : Unknown (0x00)
Enable Status        : disabled

User ID              : 10
User Name            :
Fixed Name           : No
Access Available     : call-in / callback
Link Authentication  : disabled
IPMI Messaging       : disabled
Privilege Level      : Unknown (0x00)
Enable Status        : disabled
`
var chGetAccess2 = `
Maximum User IDs     : 15
Enabled User IDs     : 7

User ID              : 1
User Name            :
Fixed Name           : Yes
Access Available     : callback
Link Authentication  : disabled
IPMI Messaging       : disabled
Privilege Level      : ADMINISTRATOR
Enable Status        : disabled

User ID              : 2
User Name            : root
Fixed Name           : Yes
Access Available     : callback
Link Authentication  : enabled
IPMI Messaging       : enabled
Privilege Level      : ADMINISTRATOR
Enable Status        : disabled

User ID              : 3
User Name            : admin
Fixed Name           : No
Access Available     : call-in / callback
Link Authentication  : enabled
IPMI Messaging       : enabled
Privilege Level      : ADMINISTRATOR
Enable Status        : enabled

User ID              : 4
User Name            : null4
Fixed Name           : No
Access Available     : call-in / callback
Link Authentication  : enabled
IPMI Messaging       : enabled
Privilege Level      : NO ACCESS
Enable Status        : enabled

User ID              : 5
User Name            : null5
Fixed Name           : No
Access Available     : call-in / callback
Link Authentication  : disabled
IPMI Messaging       : disabled
Privilege Level      : NO ACCESS
Enable Status        : disabled

User ID              : 6
User Name            :
Fixed Name           : No
Access Available     : call-in / callback
Link Authentication  : disabled
IPMI Messaging       : disabled
Privilege Level      : NO ACCESS
Enable Status        : disabled

User ID              : 7
User Name            :
Fixed Name           : No
Access Available     : call-in / callback
Link Authentication  : disabled
IPMI Messaging       : disabled
Privilege Level      : NO ACCESS
Enable Status        : disabled

User ID              : 8
User Name            :
Fixed Name           : No
Access Available     : call-in / callback
Link Authentication  : disabled
IPMI Messaging       : disabled
Privilege Level      : NO ACCESS
Enable Status        : disabled

User ID              : 9
User Name            :
Fixed Name           : No
Access Available     : call-in / callback
Link Authentication  : disabled
IPMI Messaging       : disabled
Privilege Level      : NO ACCESS
Enable Status        : disabled

User ID              : 10
User Name            :
Fixed Name           : No
Access Available     : call-in / callback
Link Authentication  : disabled
IPMI Messaging       : disabled
Privilege Level      : NO ACCESS
Enable Status        : disabled

User ID              : 11
User Name            :
Fixed Name           : No
Access Available     : call-in / callback
Link Authentication  : disabled
IPMI Messaging       : disabled
Privilege Level      : NO ACCESS
Enable Status        : disabled

User ID              : 12
User Name            :
Fixed Name           : No
Access Available     : call-in / callback
Link Authentication  : disabled
IPMI Messaging       : disabled
Privilege Level      : NO ACCESS
Enable Status        : disabled

User ID              : 13
User Name            :
Fixed Name           : No
Access Available     : call-in / callback
Link Authentication  : disabled
IPMI Messaging       : disabled
Privilege Level      : NO ACCESS
Enable Status        : disabled

User ID              : 14
User Name            :
Fixed Name           : No
Access Available     : call-in / callback
Link Authentication  : disabled
IPMI Messaging       : disabled
Privilege Level      : NO ACCESS
Enable Status        : disabled

User ID              : 15
User Name            :
Fixed Name           : No
Access Available     : call-in / callback
Link Authentication  : disabled
IPMI Messaging       : disabled
Privilege Level      : NO ACCESS
Enable Status        : disabled
`

//ipmitool channel info
var chInfo = `Channel 0x1 info:
  Channel Medium Type   : 802.3 LAN
  Channel Protocol Type : IPMB-1.0
  Session Support       : multi-session
  Active Session Count  : 1
  Protocol Vendor ID    : 7154
  Volatile(active) Settings
    Alerting            : enabled
    Per-message Auth    : enabled
    User Level Auth     : enabled
    Access Mode         : always available
  Non-Volatile Settings
    Alerting            : enabled
    Per-message Auth    : enabled
    User Level Auth     : enabled
    Access Mode         : always available
`

//func (ch Channel) parseChannelInfo(data string) error
func TestChannelInfo(t *testing.T) {
	ch := Channel{Id: 1}
	err := ch.parseChannelInfo(chInfo)
	if err != nil {
		t.Errorf("%s", err)
	}
	if !ch.IsLan() {
		t.Errorf("IsLan() should be true: %v", ch)
	}
}

//func parseChannelUserInfo(data string) (users IpmiUsers, err error)
func TestChannelUserInfo(t *testing.T) {
	users, err := parseChannelUserInfo(chGetAccess1)
	if err != nil {
		t.Errorf("%s", err)

	}
	if len(users) != 10 {
		t.Errorf("wrong qty %d", len(users))
	}
	admins := users.OnlyEnabled().OnlyPrivileged().OnlyNamedAdmin()
	if len(admins) != 1 {
		for _, a := range admins {
			t.Logf("%#v\n", a)
		}
		t.Errorf("expected 1 admin, got %d", len(admins))
	}
	users, err = parseChannelUserInfo(chGetAccess2)
	if err != nil {
		t.Errorf("%s", err)

	}
	if len(users) != 15 {
		t.Errorf("wrong qty %d", len(users))
	}
	admins = users.OnlyEnabled().OnlyPrivileged().OnlyNamedAdmin()
	if len(admins) != 1 {
		for _, a := range admins {
			t.Logf("%#v\n", a)
		}
		t.Errorf("expected 1 admin, got %d", len(admins))
	}
}
