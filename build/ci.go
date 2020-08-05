// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

// +build mage

package main

import (
	"context"
	"os"
	fp "path/filepath"

	"github.com/magefile/mage/mg"

	"github.com/purecloudlabs/gprovision/build/paths"
)

//targets for CI to run

/* extracting human-readable 'go test' output from ci console
-------------
* open the page for the build in question
* click "View as plain text" on the left, just under "Console Output"
* right click, save as...
* open the saved file in an editor
* trim non-json from beginning and end, save with name like console-trimmed.json
* now run:
pkgs=$(cat console-trimmed.json|jq -r 'select(.Action=="fail")|.Package'|sort -u)
for p in $pkgs; do
    cat console-trimmed.json |\
	jq -r "select(.Package==\"$p\")|select(.Output)|.Output"
done | grep -v ^$ > test-out.txt
*/

type CI mg.Namespace

func (CI) UnitTestStage(ctx context.Context) {
	out := fp.Join(os.Getenv("WORKSPACE"), "unit_test_out.xml")
	newctx := context.WithValue(ctx, "JUNIT", out)
	mg.CtxDeps(newctx, Tests.Unit, Tests.Lint)
}

func (CI) BuildStage(ctx context.Context) {
	mg.CtxDeps(ctx, BuildAll)
}

func (CI) IntegTestStage(ctx context.Context) {
	integxml := fp.Join(os.Getenv("WORKSPACE"), "integ_test_out.xml")
	integctx := context.WithValue(ctx, "JUNIT", integxml)
	mg.CtxDeps(integctx, Tests.Integ)

	lcxml := fp.Join(os.Getenv("WORKSPACE"), "lifecycle_test_out.xml")
	lcctx := context.WithValue(ctx, "JUNIT", lcxml)
	mg.CtxDeps(lcctx, Tests.Lifecycle)
}

func (CI) Artifacts(ctx context.Context) error {
	err := os.Mkdir(paths.ArtifactDir, 0755)
	if err != nil {
		return err
	}
	in := []string{paths.KBoot, paths.KMfg, paths.ImgAppsTxz}
	in = append(in, must(fp.Glob(fp.Join(paths.WorkDir, "*.exe")))...)

	for _, fname := range in {
		newname := fp.Join(paths.ArtifactDir, fp.Base(fname))
		err = os.Rename(fname, newname)
		if err != nil {
			return err
		}
	}
	return nil
}

func must(s []string, err error) []string {
	if err != nil {
		panic(err)
	}
	return s
}
