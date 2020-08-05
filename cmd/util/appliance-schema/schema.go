// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

// Appliance-schema generates a json schema for github.com/purecloudlabs/gprovision/pkg/appliance or
// github.com/purecloudlabs/gprovision/pkg/mfg/qa.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/purecloudlabs/gprovision/pkg/appliance"

	"github.com/alecthomas/jsonschema"
)

const Warn = `WARNING:
	schema will need to be hand-edited, as the output isn't perfect
	* jsonschema doesn't realize that certain types are marshalled as strings
	* jsonschema assumes no additional properties are allowed (but properties
		are sometimes used as comments)
`

func main() {
	dofacts := flag.Bool("facts", false, "produce schema for platform_facts.json rather than appliance.json")
	flag.Parse()
	fmt.Fprint(os.Stderr, Warn)
	var schem *jsonschema.Schema
	if *dofacts {
		schem = jsonschema.Reflect(&appliance.PlatFacts{})
	} else {
		type root struct{ Variants []appliance.Variant_ }
		schem = jsonschema.Reflect(&root{})
	}
	data, err := json.MarshalIndent(schem, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		return
	}
	fmt.Printf("%s\n", data)
}
