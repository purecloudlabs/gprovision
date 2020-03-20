// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

//Package id returns UID, GID for given user from given filesystem
//(not necessarily mounted at /)
package id

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

//return numeric group id of 'group', using data in fs at 'root'
// or -1 if error
func GetGID(root, group string) (rv int, err error) {
	rv = -1
	gfile := filepath.Join(root, "etc", "group")
	grps, err := os.Open(gfile)
	if err != nil {
		return
	}
	scanner := bufio.NewScanner(grps)
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), ":")
		if fields[0] == group {
			rv, err = strconv.Atoi(fields[2])
			if err != nil {
				err = fmt.Errorf("getGID: err %s finding group %s in %s", err, group, gfile)
				rv = -1
			}
			return
		}
	}
	err = fmt.Errorf("getGID: can't find group %s in %s", group, gfile)
	return
}

//return numeric user id of 'user', using data in fs at 'root'
// or -1 if error
func GetUID(root, user string) (rv int, err error) {
	rv = -1
	ufile := filepath.Join(root, "etc", "passwd")
	users, err := os.Open(ufile)
	if err != nil {
		return
	}
	scanner := bufio.NewScanner(users)
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), ":")
		if fields[0] == user {
			rv, err = strconv.Atoi(fields[2])
			if err != nil {
				err = fmt.Errorf("getUID: err %s finding user %s in %s", err, user, ufile)
				rv = -1
			}
			return
		}
	}
	err = fmt.Errorf("getUID: can't find user %s in %s", user, ufile)
	return
}
