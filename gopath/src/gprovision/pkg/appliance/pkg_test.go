// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package appliance

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	fp "path/filepath"
	"testing"

	"github.com/santhosh-tekuri/jsonschema"
)

//load json from disk, compare with bindata version
func TestBindataProprietary(t *testing.T) {
	aj := fp.Join(os.Getenv("INFRA_ROOT"), "gopath/src/gprovision/proprietary/data/appliance/appliance.json")
	if _, err := os.Stat(aj); err != nil {
		t.Skipf("no json to embed, nothing to compare")
	}
	f, err := ioutil.ReadFile(aj)
	if err != nil {
		t.Errorf("loading appliance.json: %s", err)
	}
	j := getJson()
	if !bytes.Equal(j, f) {
		t.Errorf("bindata and file don't match")
	}
}

//check that json is compatible with our struct
func TestUnmarshal(t *testing.T) {
	j := getJson()
	err := loadJson(j)
	if err != nil {
		t.Errorf("loading default json: %s", err)
	}
}

func TestPersistence(t *testing.T) {
	dir, err := ioutil.TempDir("", "gotestjson")
	if err != nil {
		t.Errorf("creating temp dir: %s", err)
	}
	defer os.RemoveAll(dir)
	j := getJson()
	if err := loadJson(j); err != nil {
		t.Error(err)
	}
	if len(variants) == 0 {
		t.Errorf("failed to load json")
	}
	for i, v := range variants {
		file := fp.Join(dir, v.DevCodeName)
		d := fp.Base(dir)
		out := &Variant{
			i: v,
			//populate these with something to ensure the values are re-loaded
			mfg:    fmt.Sprintf("mfg%d%s", i, d[10:]),
			prod:   fmt.Sprintf("prod%d%s", i, d[10:]),
			sku:    fmt.Sprintf("sku%d%s", i, d[10:]),
			serial: fmt.Sprintf("ser%d%s", i, d[10:]),
		}
		out.write(file)
		in := read(file)
		outStr := fmt.Sprintf("%#v", out)
		inStr := fmt.Sprintf("%#v", in)
		if inStr != outStr {
			t.Errorf("want %s\ngot  %s", outStr, inStr)
		}
	}
}

//test against the appliance schema
func TestApplianceJsonConformance(t *testing.T) {
	schema, err := jsonschema.Compile("schemas/appliance.json")
	if err != nil {
		t.Error(err)
		return
	}
	t.Run("default", func(t *testing.T) {
		f := bytes.NewReader([]byte(aj_default))
		err = schema.Validate(f)
		if err != nil {
			t.Error(err)
		}
	})
	t.Run("embedded", func(t *testing.T) {
		j, err := Asset("appliance.json")
		if err != nil {
			t.Skip("embedded appliance.json not present")
		}
		f := bytes.NewReader(j)
		err = schema.Validate(f)
		if err != nil {
			t.Error(err)
		}

	})
}

//test against the platform_facts schema
func TestPlatFactsJsonConformance(t *testing.T) {
	schema, err := jsonschema.Compile("schemas/platform_facts.json")
	if err != nil {
		t.Error(err)
		return
	}
	type tstdata struct {
		name string
		json []byte
	}
	testdata := []tstdata{
		{name: "default", json: []byte(aj_default)},
	}
	j, err := Asset("appliance.json")
	if err == nil {
		testdata = append(testdata, tstdata{name: "embedded", json: j})
	}

	for _, td := range testdata {
		t.Run(td.name, func(t *testing.T) {
			//j := getJson()
			if err := loadJson(td.json); err != nil {
				t.Error(err)
			}
			for _, v := range variants {
				t.Run(v.DevCodeName, func(t *testing.T) {
					out := &Variant{
						i:      v,
						mfg:    "mfg",
						prod:   "prod",
						sku:    "sku",
						serial: "serial",
					}
					j := out.json()
					reader := bytes.NewReader(j)
					err = schema.Validate(reader)
					if err != nil {
						t.Error(err)
						t.Logf("json in question: %s\n", j)
					}
				})
			}
		})
	}
}
