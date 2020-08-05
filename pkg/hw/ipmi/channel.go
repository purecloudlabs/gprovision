// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package ipmi

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/purecloudlabs/gprovision/pkg/log"
)

type Channel struct {
	Id     int
	Medium string
	Users  IpmiUsers
}
type Channels []*Channel

func GetChannels() (chs Channels) {
	for i := 0; i < 16; i++ {
		ch := &Channel{Id: i}
		err := ch.getInfo()
		if err == nil {
			chs = append(chs, ch)
		}
	}
	return
}

func (chs Channels) FirstLAN() *Channel {
	for _, c := range chs {
		if c.IsLan() {
			return c
		}
	}
	return nil
}
func (ch *Channel) getInfo() error {
	cmd := exec.Command("ipmitool", "channel", "info", fmt.Sprintf("%d", ch.Id))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}
	return ch.parseChannelInfo(string(out))
}
func (ch *Channel) parseChannelInfo(data string) error {
	lines := strings.Split(data, "\n")
	if len(lines) < 2 ||
		!strings.HasPrefix(lines[0], "Channel ") ||
		!strings.HasSuffix(lines[0], " info:") {
		return os.ErrInvalid
	}
	for i := 1; i < len(lines); i++ {
		split := strings.Split(lines[i], ":")
		if len(split) != 2 {
			continue
		}
		if strings.TrimSpace(split[0]) == "Channel Medium Type" {
			ch.Medium = strings.TrimSpace(split[1])
			//this is currently the only property we care about, so stop processing lines
			break
		}
	}
	return nil
}

func (ch Channel) IsLan() bool {
	return ch.Medium == "802.3 LAN"
}

func (ch *Channel) GetUsers() error {
	cmd := exec.Command("ipmitool", "channel", "getaccess", fmt.Sprintf("%d", ch.Id))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}
	ch.Users, err = parseChannelUserInfo(string(out))
	return err
}

func parseChannelUserInfo(data string) (users IpmiUsers, err error) {
	lines := strings.Split(data, "\n")
	var user *IpmiUser
	for _, line := range lines {
		split := strings.Split(line, ":")
		if len(split) != 2 {
			continue
		}
		key := strings.TrimSpace(split[0])
		val := strings.TrimSpace(split[1])
		switch key {
		case "User ID":
			//this is always first - start new record
			if user != nil {
				users = append(users, user)
			}
			user = new(IpmiUser)
			user.Id, err = strconv.ParseInt(val, 10, 64)
			if err != nil {
				return nil, err
			}
		case "User Name":
			user.Name = val
		case "Link Authentication":
			user.LinkAuth = enaDisaToBool(val)
		case "IPMI Messaging":
			user.IpmiMsg = enaDisaToBool(val)
		case "Privilege Level":
			user.Privilege = val
		case "Enable Status":
			user.Enabled = enaDisaToBool(val)
		}
	}
	if user != nil {
		users = append(users, user)
	}
	return
}
func enaDisaToBool(s string) bool {
	if strings.HasPrefix(strings.ToLower(s), "enable") {
		return true
	}
	if !strings.HasPrefix(strings.ToLower(s), "disable") {
		log.Logf("parse error - %s does not match 'enable' or 'disable'")
	}
	return false
}
