// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package ipmi

import (
	"testing"

	"github.com/purecloudlabs/gprovision/pkg/log/testlog"
)

const (
	//ipmitool -H 10.155.8.180 -U admin -P X9WmW0UF lan print 1
	out = `Set in Progress         : Set Complete
Auth Type Support       : MD5 PASSWORD
Auth Type Enable        : Callback : MD5 PASSWORD
                        : User     : MD5 PASSWORD
                        : Operator : MD5 PASSWORD
                        : Admin    : MD5 PASSWORD
                        : OEM      :
IP Address Source       : DHCP Address
IP Address              : 10.155.8.180
Subnet Mask             : 255.255.255.0
MAC Address             : 00:26:fd:a0:0d:52
SNMP Community String   : public
IP Header               : TTL=0x40 Flags=0x40 Precedence=0x00 TOS=0x10
BMC ARP Control         : ARP Responses Enabled, Gratuitous ARP Enabled
Gratituous ARP Intrvl   : 2.0 seconds
Default Gateway IP      : 10.155.8.1
Default Gateway MAC     : 00:00:00:00:00:00
Backup Gateway IP       : 0.0.0.0
Backup Gateway MAC      : 00:00:00:00:00:00
802.1q VLAN ID          : Disabled
802.1q VLAN Priority    : 0
RMCP+ Cipher Suites     : 0,1,2,3,4,6,7,8,9,11,12,13,15,16,17,18
Cipher Suite Priv Max   : caaaaaaaaaaaaaa
                        :     X=Cipher Suite Unused
                        :     c=CALLBACK
                        :     u=USER
                        :     o=OPERATOR
                        :     a=ADMIN
                        :     O=OEM
Bad Password Threshold  : Not Available
`
)

//func LogMacs()
func TestLogMacs(t *testing.T) {
	tl := testlog.NewTestLog(t, true, false)
	var ch Channel
	ch.Id = 0
	m, err := ch.mac(out, true)
	if err != nil {
		t.Error(err)
	}
	tl.Freeze()
	l := tl.Buf.String()
	if l != "" {
		t.Log(l)
	}
	want := "00:26:fd:a0:0d:52"
	if m != want {
		t.Errorf("got %s, want %s", m, want)
	}
}
