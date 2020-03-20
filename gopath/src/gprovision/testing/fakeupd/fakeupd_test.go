// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package fakeupd

import (
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"
	fp "path/filepath"
	"regexp"
	"strings"
	"testing"
)

// func Make(tmpdir, kern, logger string) ([]byte, error)
// ensures the target binaries will compile, and that the resulting update's
// content is reasonable.
func TestFakeupd(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "go-test-fakeupd")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if t.Failed() {
			t.Logf("not deleting %s", tmpdir)
			return
		}
		err = os.RemoveAll(tmpdir)
		if err != nil {
			t.Fatal(err)
		}
	}()
	kern := fp.Join(tmpdir, "kern")
	err = ioutil.WriteFile(kern, []byte("fake kernel"), 0644)
	if err != nil {
		t.Fatal(err)
	}
	data, err := Make(tmpdir, kern, "")
	if err != nil {
		t.Fatal(err)
	}
	untar := exec.Command("tar", "tvJ")
	untar.Stdin = bytes.NewReader(data)
	out, err := untar.Output()
	if err != nil {
		t.Error(err)
		t.Log(string(out))
	}
	expectedContent := []string{
		"^drwxrwxrwx .* boot/$",
		"^drwxrwxrwx .* etc/$",
		"^drwxrwxrwx .* etc/systemd/$",
		"^drwxrwxrwx .* dev/$",
		"^drwxrwxrwx .* usr/$",
		"^drwxrwxrwx .* usr/lib/$",
		"^drwxrwxrwx .* usr/lib/systemd/$",
		"^-rw-r--r-- .* boot/norm_boot$",
		//ensures that one of the two has substantial size ('[0-9]{6,8}')
		"-rwxr-xr-x 0/0  *[0-9]{6,8} [0-9- :]{12,16} (usr/lib/systemd/systemd|bin/chpasswd)$",
		//ensures a symlink exists from one name to the other
		"l--------- 0/0 * 0 [0-9- :]{12,16} (usr/lib/systemd/systemd|bin/chpasswd) -> /(usr/lib/systemd/systemd|bin/chpasswd)$",
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	checkREs(t, expectedContent, lines)
	if t.Failed() {
		err = ioutil.WriteFile(fp.Join(tmpdir, "fake.upd.txz"), data, 0644)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func checkREs(t *testing.T, wantREs, got []string) {
	t.Helper()
	for _, want := range wantREs {
		found := false
		re := regexp.MustCompile(want)
		for i := range got {
			if re.MatchString(got[i]) {
				found = true
				got = append(got[:i], got[i+1:]...)
				break
			}
		}
		if !found {
			t.Errorf("no match found for %q in\n %#v", want, got)
		}
	}
}
