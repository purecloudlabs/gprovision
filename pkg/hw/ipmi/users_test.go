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

var raw [2][]byte

func init() {
	//ID,Name,Callin,Link Auth,IPMI Msg,Channel Priv Limit
	raw[0] = []byte(`1,,false,false,false,ADMINISTRATOR
2,root,false,true,true,ADMINISTRATOR
3,admin,true,true,true,ADMINISTRATOR
4,null4,true,false,false,NO ACCESS
5,null5,true,false,false,NO ACCESS
6,,true,false,false,NO ACCESS
7,,true,false,false,NO ACCESS
8,,true,false,false,NO ACCESS
9,,true,false,false,NO ACCESS
10,,true,false,false,NO ACCESS
11,,true,false,false,NO ACCESS
12,,true,false,false,NO ACCESS
13,,true,false,false,NO ACCESS
14,,true,false,false,NO ACCESS
15,,true,false,false,NO ACCESS
`)
	raw[1] = []byte(`1,,true,false,false,Unknown (0x00)
2,ADMIN,false,false,true,ADMINISTRATOR
3,,true,false,false,Unknown (0x00)
4,,true,false,false,Unknown (0x00)
5,,true,false,false,Unknown (0x00)
6,,true,false,false,Unknown (0x00)
7,,true,false,false,Unknown (0x00)
8,,true,false,false,Unknown (0x00)
9,,true,false,false,Unknown (0x00)
10,,true,false,false,Unknown (0x00)
`)
}

//func parseUsers(out []byte) (users IpmiUsers)
func TestParseUsers(t *testing.T) {
	users := parseUsers(raw[0])
	if len(users) != 15 {
		t.Errorf("wrong number of users")
	}
	if users[0].Name != "" || users[1].Name != "root" || users[2].Id != 3 {
		t.Logf("in:\n%s", string(raw[0]))
		t.Logf("out:")
		for i, u := range users {
			t.Logf("%d: %#v", i, u)
		}
		t.Errorf("parse error")
	}
	users = parseUsers(raw[1])
	if len(users) != 10 {
		t.Errorf("wrong number of users")
	}
	if users[0].Name != "" || users[1].Name != "ADMIN" || users[2].Id != 3 {
		t.Logf("in:\n%s", string(raw[1]))
		t.Logf("out:")
		for i, u := range users {
			t.Logf("%d: %#v", i, u)
		}
		t.Errorf("parse error")
	}
	users = parseUsers([]byte{})
	if len(users) != 0 {
		t.Errorf("parse error for 0-length output")
	}
}

//func (usrs IpmiUsers) OnlyEnabled() (enabled IpmiUsers)
func TestOnlyEnabled(t *testing.T) {
	users0 := parseUsers(raw[0])
	enabled0 := users0.OnlyEnabled()
	if len(enabled0) != 0 {
		t.Errorf("got %d, want 0", len(enabled0))
	}
	users0[1].Enabled = true
	users0[5].Enabled = true
	enabled0 = users0.OnlyEnabled()
	if len(enabled0) != 2 {
		t.Errorf("got %d, want 2", len(enabled0))
	}

	users1 := parseUsers(raw[1])
	enabled1 := users1.OnlyEnabled()
	if len(enabled1) != 0 {
		t.Errorf("got %d, want 0", len(enabled1))
	}
}

//func (usrs IpmiUsers) OnlyPrivileged() (admin IpmiUsers)
func TestOnlyPrivileged(t *testing.T) {
	users := parseUsers(raw[0])
	privileged := users.OnlyPrivileged()
	for _, u := range privileged {
		if !u.HasAdminPriv() {
			t.Errorf("user not privileged: %#v", u)
		}
	}
	if len(privileged) != 3 {
		t.Errorf("got %d, want 3", len(privileged))
	}
	users = parseUsers(raw[1])
	privileged = users.OnlyPrivileged()
	for _, u := range privileged {
		if !u.HasAdminPriv() {
			t.Errorf("user not privileged: %#v", u)
		}
	}
	if len(privileged) != 1 {
		t.Errorf("got %d, want 1", len(privileged))
	}
	users = nil
	privileged = users.OnlyPrivileged()
	if len(privileged) != 0 {
		t.Errorf("got %d, want 0", len(privileged))
	}
}

//func (usrs IpmiUsers) OnlyNamed() (named IpmiUsers)
func TestOnlyNamed(t *testing.T) {
	users := parseUsers(raw[0])
	named := users.OnlyNamed()
	if len(named) != 2 {
		t.Error("wrong number of users")
	}
	users = nil
	named = users.OnlyNamed()
	if len(named) != 0 {
		t.Error("wrong number of users")
	}
}

//func (usrs IpmiUsers) OnlyNamedAdmin()(admins IpmiUsers)
func TestOnlyNamedAdmin(t *testing.T) {
	users0 := parseUsers(raw[0])
	users1 := parseUsers(raw[1])
	a0 := users0.OnlyNamedAdmin()
	if len(a0) != 1 {
		t.Error()
	}
	a1 := users1.OnlyNamedAdmin()
	if len(a1) != 1 {
		t.Error()
	}
	only0 := a0.OnlyPrivileged()
	if len(only0) != 1 {
		t.Error()
	}
	only1 := a1.OnlyPrivileged()
	if len(only1) != 1 {
		t.Error()
	}
}
