// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package nic

import (
	"os"
	"testing"
)

var nics []Nic

//requires human intervention to verify output is correct... not sure how to automate
func TestFindIRQs(t *testing.T) {
	nics = List()
	if len(nics) == 0 {
		t.Errorf("0 nics, can't test")
		return
	}
	i, err := nics[0].FindIRQs()
	if err != nil {
		_, onJenkins := os.LookupEnv("JENKINS_NODE_COOKIE")
		if onJenkins && os.IsNotExist(err) {
			t.Skipf("jenkins has no visible IRQs")
		}
		t.Errorf("%s", err)
	}
	if i == 0 {
		t.Errorf("0 irqs")
	}
	irqs := nics[0].ListIRQs()
	if len(irqs) == 0 {
		t.Errorf("0 irqs")
	}
	t.Log(irqs)
}

func TestListIRQs(t *testing.T) {
	l := nics[0].ListIRQs()
	if len(l) == 0 {
		_, onJenkins := os.LookupEnv("JENKINS_NODE_COOKIE")
		if onJenkins {
			t.Skipf("jenkins has no visible IRQs")
		}
		t.Errorf("no IRQs!")
	} else {
		t.Logf("%v", l)
	}
}
