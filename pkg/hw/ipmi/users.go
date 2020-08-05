// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package ipmi

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"

	"github.com/purecloudlabs/gprovision/pkg/log"
)

type IpmiUser struct {
	Id                int64
	Name              string
	LinkAuth, IpmiMsg bool
	Privilege         string
	Enabled           bool
	//Callin bool //don't care
}
type IpmiUsers []*IpmiUser

/* List users.
   WARNING: cannot tell whether a user is enabled
   on a given channel - so don't use IsEnabled or
   OnlyEnabled on returned list
*/
func ListUsers() (users IpmiUsers) {
	cmd := exec.Command("ipmitool", "-c", "user", "list")
	out, err := cmd.Output()
	if err != nil {
		log.Logf("failed to execute %v: %s", cmd.Args, err)
	}
	return parseUsers(out)
}

func parseUsers(out []byte) (users IpmiUsers) {
	r := csv.NewReader(bytes.NewBuffer(out))
	for {
		user := new(IpmiUser)
		/*
		   ID,Name,Callin,Link Auth,IPMI Msg,Channel Priv Limit
		   1,,true,false,false,Unknown (0x00)
		   2,ADMIN,false,false,true,ADMINISTRATOR

		   1,,false,false,false,ADMINISTRATOR
		   2,root,false,true,true,ADMINISTRATOR
		   3,admin,true,true,true,ADMINISTRATOR
		   4,null4,true,false,false,NO ACCESS
		*/
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err == nil && len(record) != 6 {
			log.Logf("list ipmi users: bad record length in %s - got %v", string(out), record)
			return nil
		}
		if err == nil {
			user.Id, err = strconv.ParseInt(record[0], 10, 64)
		}
		if err == nil {
			user.Name = record[1]
			//user.Callin, err = strconv.ParseBool(record[2])
			//}
			//if err == nil {
			user.LinkAuth, err = strconv.ParseBool(record[3])
		}
		if err == nil {
			user.IpmiMsg, err = strconv.ParseBool(record[4])
		}
		if err != nil {
			log.Logf("list ipmi users: error %s processing output %s line %v", err, string(out), record)
			return nil
		}
		user.Privilege = record[5]
		users = append(users, user)
	}
	return
}

func (u IpmiUser) HasAdminPriv() bool {
	return u.Privilege == "ADMINISTRATOR"
}

//only works for user info from GetChannels()
func (u IpmiUser) IsEnabled() bool {
	return u.Enabled
}

func (u IpmiUser) GuessEnabled() bool {
	//IpmiMsg _appears_ to correspond with being enabled... however there's
	// also the notion of an account being enabled for a particular channel
	return u.IpmiMsg && u.Privilege != "NO ACCESS" && u.Privilege != "Unknown (0x00)"
}

//filter out accounts that aren't enabled
func (usrs IpmiUsers) OnlyEnabled() (enabled IpmiUsers) {
	for _, u := range usrs {
		if u.IsEnabled() {
			enabled = append(enabled, u)
		}
	}
	return
}

//filter out accounts without admin privileges
func (usrs IpmiUsers) OnlyPrivileged() (privileged IpmiUsers) {
	for _, u := range usrs {
		if u.HasAdminPriv() {
			privileged = append(privileged, u)
		}
	}
	return
}

//filter out unnamed and null accounts
func (usrs IpmiUsers) OnlyNamed() (named IpmiUsers) {
	for _, u := range usrs {
		if u.Name != "" && !strings.HasPrefix(strings.ToLower(u.Name), "null") {
			named = append(named, u)
		}
	}
	return
}

//filter out accounts except those named "ADMIN","admin","Administrator", etc
func (usrs IpmiUsers) OnlyNamedAdmin() (admins IpmiUsers) {
	for _, u := range usrs {
		if strings.HasPrefix(strings.ToLower(u.Name), "admin") {
			admins = append(admins, u)
		}
	}
	return
}

func (u IpmiUser) SetPassword(pw string) (err error) {
	cmd := exec.Command("ipmitool", "user", "set", "password", fmt.Sprintf("%d", u.Id), pw)
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Logf("failed to set password: %s\nout: %s", err, out)
	}
	successStr := "Set User Password command successful"
	if !strings.Contains(string(out), successStr) {
		log.Logf("%v: expected '%s'\ngot %s", cmd.Args, successStr, string(out))
		err = fmt.Errorf("no error reported by %v but didn't get expected output", cmd.Args)
	}
	if err == nil {
		log.Logf("ipmi user %d password set", u.Id)
	}
	return
}

func (u *IpmiUser) SetName(name string) (err error) {
	if name == u.Name {
		log.Logf("ipmi user %d name is already %s, not trying to set", u.Id, u.Name)
		return
	}
	cmd := exec.Command("ipmitool", "user", "set", "name", fmt.Sprintf("%d", u.Id), name)
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Logf("failed to set name: %s\nout: %s", err, out)
	} else {
		u.Name = name
		log.Logf("ipmi user %d name set to %s", u.Id, u.Name)
	}
	return
}
