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
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	fp "path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/magefile/mage/mg"

	"github.com/purecloudlabs/gprovision/build/paths"
	gtst "github.com/purecloudlabs/gprovision/testing"
)

/* Env vars
RUN - passed to go test -run. Only tests that match the given regex will run.
    Overridden in some cases.
COUNT - passed to go test -count. Use 1 to bypass test result caching, and
    higher values to repeat tests.
RUN and COUNT are used in testArgs() function.

UROOT_QEMU - if set, overrides default. Useful to run tests using kvm for speed,
    for example. Used in qemuEnv().
*/

type Tests mg.Namespace

//runs unit tests
func (Tests) Unit(ctx context.Context) error {
	mg.CtxDeps(ctx, Bins.Generate, libudev)

	args, err := testArgs(ctx, nil, "")
	if err != nil {
		return err
	}
	return gotest(ctx, libudevEnv(), args...)
}

// Runs stand-alone integ tests, in no particular order. Note - excludes any
// test whose name matches TestL, not just TestLifecycle_*
func (Tests) Integ(ctx context.Context) error {
	mg.CtxDeps(ctx, Kernel.Noinitramfs)
	if !gtst.OnCI() {
		//on ci, these are already built and known to be up-to-date
		mg.CtxDeps(ctx, Initramfs.Combined_cpio, Bins.Generate)
		mg.CtxDeps(ctx, Initramfs.Boot, Initramfs.Mfg)
	}
	mg.CtxDeps(ctx, qemu)
	env := qemuEnv(paths.KNoInitramfs, false)
	args, err := integArgs(ctx, "Test[^L][^i]")
	if err != nil {
		return err
	}
	return gotest(ctx, env, args...)
}

// Runs integ test matching RUN env var.
func (Tests) OneInteg(ctx context.Context) error {
	if _, ok := os.LookupEnv("RUN"); !ok {
		fmt.Println("env var RUN must be set")
		os.Exit(1)
	}
	mg.CtxDeps(ctx, Kernel.Noinitramfs, Initramfs.Combined_cpio, Bins.Generate, qemu)
	mg.CtxDeps(ctx, Initramfs.Boot, Initramfs.Mfg)
	env := qemuEnv(paths.KNoInitramfs, false)
	args, err := integArgs(ctx, "")
	if err != nil {
		return err
	}
	return gotest(ctx, env, args...)
}

// Like Integ, but simulates unit lifecycle - manufacture, factory restore,
// normal boot. Creates a "minimum viable" image to test with.
func (Tests) Lifecycle(ctx context.Context) {
	mg.CtxDeps(ctx, Tests.LifecycleLegacy)
	mg.CtxDeps(ctx, Tests.LifecycleUefi)
}

func (Tests) LifecycleLegacy(ctx context.Context) error { return lifecycleTest(ctx, false) }

func (Tests) LifecycleUefi(ctx context.Context) error { return lifecycleTest(ctx, true) }

func lifecycleTest(ctx context.Context, uefi bool) error {
	if !gtst.OnCI() {
		//on ci, kernels are already built and known to be up-to-date
		mg.CtxDeps(ctx, Kernel.Linuxmfg, Kernel.Boot)
	}
	mg.CtxDeps(ctx, qemu)

	env := qemuEnv(paths.KMfg, uefi)
	lc_img := fmt.Sprintf("%s=%s", "LC_IMG", paths.FakeUpdate)
	fmt.Println(lc_img) //other env vars are printed by qemuEnv
	env = append(env, lc_img)
	tst := "Lifecycle_Legacy"
	if uefi {
		tst = "Lifecycle_UEFI"
	}
	args, err := integArgs(ctx, tst)
	if err != nil {
		return err
	}
	return gotest(ctx, env, args...)
}

//args for 'go test', for integ tests
func integArgs(ctx context.Context, onlyRun string) ([]string, error) {
	pkgs := []string{fp.Join(paths.ImportPath, "testing/integ/")}
	prop := "proprietary/testing/integ/"
	_, err := os.Stat(fp.Join(paths.RepoRoot, prop))
	if err == nil {
		pkgs = append(pkgs, fp.Join(paths.ImportPath, prop))
	}
	return testArgs(ctx, pkgs, onlyRun)
}

//args for 'go test': pkg, -run, -count, -timeout
func testArgs(ctx context.Context, pkgs []string, onlyRun string) ([]string, error) {
	var hasDeadline bool
	var deadline time.Time
	if len(pkgs) == 0 {
		pkgs = paths.GoDirs
	}
	args := []string{}
	//pass timeout arg?
	deadline, hasDeadline = ctx.Deadline()
	if hasDeadline {
		dur := time.Until(deadline) - 20*time.Second //less time than the exact deadline so go test can print out message about what test it's on
		if dur < 0 {
			//already past deadline
			return nil, mg.Fatal(1, "deadline exceeded")
		}
		args = append(args, "-timeout", dur.String())
	}
	args = append(args, pkgs...)

	//limit tests to be run
	if onlyRun != "" {
		args = append(args, "-run", onlyRun)
	} else if run := os.Getenv("RUN"); run != "" {
		args = append(args, "-run", run)
	}

	//run test(s) multiple times
	if count := os.Getenv("COUNT"); count != "" {
		c, err := strconv.Atoi(count)
		if err != nil {
			return nil, mg.Fatalf(3, "COUNT must be unset or numeric: %s", err)
		}
		if c > 0 {
			args = append(args, "-count", count)
		}
		//go test timeout is cumulative, default 10m - enough for 2-3 runs
		if !hasDeadline && c > 2 {
			//no deadline was specified for mage
			//set timeout higher than the default - allow 4m per run
			args = append(args, "-timeout", fmt.Sprintf("%dm", c*4))
		}
	}
	return args, nil
}

//env vars needed to build/test/lint anything linking libudev
func libudevEnv() []string {
	return []string{
		"CGO_ENABLED=1",
		"CGO_CFLAGS=-I" + paths.WorkDir,
		"CGO_LDFLAGS=-L" + paths.WorkDir,
		"LD_LIBRARY_PATH=" + paths.WorkDir,
	}
}

func gotest(ctx context.Context, env []string, args ...string) error {
	//if this is set, run gotestsum and write output to the file named in xout
	junitOut := ctx.Value("JUNIT")

	env = append(env, os.Environ()...)

	var err error
	var out []byte
	if junitOut != nil {
		jout := junitOut.(string)
		gts := exec.CommandContext(ctx, "gotestsum", "--junitfile", jout, "--")
		//args after the -- are passed to go test
		for _, a := range args {
			gts.Args = append(gts.Args, a)
		}
		gts.Env = env
		fmt.Printf("running %v...\n", gts.Args)
		out, err = gts.CombinedOutput()
		if err == nil {
			return nil
		}
		fmt.Printf("%v exited with error %q. output:\n%s\n", gts.Args, err, string(out))
		if fi, serr := os.Stat(jout); serr == nil && fi.Size() > 100 {
			data, err := ioutil.ReadFile(jout)
			if err == nil && !strings.Contains(string(data), "TestMain") {
				// gotestsum exited with error, but its output file exists. trust that it
				// contains sufficient info to diagnose the issue.
				// if we return an error, ci won't parse the test report.
				// TestMain check is for weird error where integ tests seem to pass
				// but xml contains message about TestMain failure.
				return mg.Fatal(4, err)
			}
		}
		fmt.Println("running 'go test' directly for a more informative error...")
	}
	tst := exec.CommandContext(ctx, "go", "test")
	if junitOut != nil {
		tst.Args = append(tst.Args, "-json")
	}
	tst.Args = append(tst.Args, args...)
	tst.Env = env
	fmt.Printf("running %v...\n", tst.Args)
	out, err = tst.CombinedOutput()
	if err == nil {
		fmt.Println("'go test' passes")
	} else {
		//parse as json, filter out output of packages that pass?
		fmt.Printf("'go test' output:\n%s\n", string(out))
		fmt.Println("if running out of space, running with TEMPDIR env var set may help")
		return mg.Fatal(5, "go test error:", err)
	}
	return err
}

func (Tests) Lint(ctx context.Context) error {
	mg.CtxDeps(ctx, Bins.Generate)
	lp, err := exec.LookPath("golangci-lint")
	if err != nil {
		fmt.Println("golangci-lint not present, downloading...")
		err = blobcp(paths.BlobstoreLinter, paths.LinterPath, false)
		if err != nil {
			return mg.Fatal(6, "error downloading golangci-lint:", err)
		}
		fmt.Println("golangci-lint: downloaded.")
		err = os.Chmod(paths.LinterPath, 0755)
		if err != nil {
			return mg.Fatal(7, "chmod of golangci-lint:", err)
		}
		lp = paths.LinterPath
	}
	// FIXME does not check files with build constraints. Would need to list
	// individual packages, filtering out a few where release builds cannot be tested.
	exprList := []string{
		"(cfa.Coord|golang.Environ|net.IPNet|netexport.StringyMac|netexport.Route).*composite literal uses unkeyed fields",
		"SA9004",
		"`(wrapBrackets|wrapCorners|unknown|endiannessSwap|testPrintComplexCfg|validateCFEX|expectLogContent|defaultRoute[46])` is unused",
		"goroutine calls T.Fatal", //should look into this one. Annoying if it is a true problem - will require sizeable rewrite to solve.
		"return value of.*Until.*not checked",
	}

	lint := exec.Command(lp, "run") //additional args set below
	//set timeout if there is one
	deadline, hasDeadline := ctx.Deadline()
	if hasDeadline {
		dur := time.Until(deadline) - 20*time.Second //less time than the exact deadline so go test can print out message about what test it's on
		if dur < 0 {
			//already past deadline
			return mg.Fatal(8, "deadline exceeded")
		}
		lint.Args = append(lint.Args, "--timeout", dur.String())
	}
	lint.Args = append(lint.Args, "./...") //will not work with abs path, so set pwd
	for _, e := range exprList {
		lint.Args = append(lint.Args, "-e", e)
	}
	lint.Env = append(libudevEnv(), os.Environ()...)
	lint.Stderr = os.Stderr
	lint.Stdout = os.Stdout
	lint.Dir = paths.RepoRoot
	err = lint.Run()
	if err != nil {
		fmt.Printf("running %v: %s\n", lint.Args, err.Error())
		return mg.Fatal(9, "golangci-lint exec:", err)
	}
	fmt.Println("golangci-lint: success")
	return nil
}
