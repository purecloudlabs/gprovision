// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

// +build mage

/*
 build file for mage build system
 list tgts with
go run magerunner.go -l

 build tgt with
go run magerunner.go tgt
*/

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	fp "path/filepath"
	"strings"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
	"github.com/magefile/mage/target"

	"github.com/purecloudlabs/gprovision/build/paths"
)

func BuildAll(ctx context.Context) error {
	fmt.Println("mage running")
	mg.CtxDeps(ctx, Kernel.Boot, Kernel.Linuxmfg, Bins.Win, Bins.ImgTxz)
	return nil
}

type Bins mg.Namespace

//binaries for windows
func (Bins) Win(ctx context.Context) error {
	mg.CtxDeps(ctx, Bins.Generate, workdir)
	env := make(map[string]string)
	env["GOOS"] = "windows"
	env["CGO_ENABLED"] = "0"
	apps, err := paths.Pkglist(paths.WinCmds...)
	if err != nil {
		return err
	}
	return buildeach(env, []string{"release", "light"}, apps...)
}

//names of cmds built in Bins.Img, for use in Bins.ImgTxz
var cmdNames []string

//binaries for img
func (Bins) Img(ctx context.Context) error {
	mg.CtxDeps(ctx, Bins.Generate, workdir, libudev)
	apps, err := paths.Pkglist(paths.ImgCmds...)
	if err != nil {
		return err
	}
	env := make(map[string]string)
	env["CGO_ENABLED"] = "1"
	env["CGO_CFLAGS"] = "-I" + paths.WorkDir
	env["CGO_LDFLAGS"] = "-L" + paths.WorkDir
	for _, app := range apps {
		cmdNames = append(cmdNames, fp.Base(app))
	}
	tags := []string{"release", "light"}
	return buildeach(env, tags, apps...)
}

//tarball of binaries for img
func (Bins) ImgTxz(ctx context.Context) error {
	mg.CtxDeps(ctx, Bins.Img)
	args := []string{"cJf", paths.ImgAppsTxz, "-C", paths.WorkDir, "--owner=0", "--group=0"}
	args = append(args, cmdNames...)
	return sh.Run("tar", args...)
}

// 2 flavors of init binary, to be embedded in kernel
func (Bins) Embedded(ctx context.Context) {
	mg.CtxDeps(ctx, Bins.NormalInit, Bins.MfgInit)
}

func (Bins) NormalInit(ctx context.Context) error {
	return Bins{}.buildInit(ctx, nil, paths.InitBin) //buildinit adds release tag
}

func (Bins) MfgInit(ctx context.Context) error {
	return Bins{}.buildInit(ctx, []string{"mfg"}, paths.MfgBin)
}

func (Bins) buildInit(ctx context.Context, addtags []string, tgt string) error {
	tags := []string{"release"}
	tags = append(tags, addtags...)
	pkg := paths.ImportPath + "/cmd/init"
	deps, err := depDirs(ctx, pkg, tags)
	if err != nil {
		return err
	}
	//check if anything is newer than tgt (if it exists)
	rebuild, err := target.Dir(tgt, deps...)
	if err != nil {
		return err
	}
	if !rebuild {
		fmt.Println("skipping build of", pkg)
		return nil
	}
	//if Bins.Generate runs before target.Dir, we will always rebuild
	mg.CtxDeps(ctx, Bins.Generate, workdir)
	env := make(map[string]string)
	env["CGO_ENABLED"] = "0"
	return build(env, "-tags", strings.Join(tags, " "), "-o", tgt, pkg)
}

// Misc utility binaries. Not used, just make sure they compile. Output is
// to GOPATH/bin since we don't specify -o.
func (Bins) Util(ctx context.Context) error {
	mg.CtxDeps(ctx, Bins.Generate)
	pkgs, err := paths.Pkglist(paths.UtilCmds...)
	if err != nil {
		return err
	}
	args := []string{"build"}
	args = append(args, pkgs...)
	return sh.Run("go", args...)
}

//run go generate
func (Bins) Generate(ctx context.Context) error {
	mg.CtxDeps(ctx, goBindata, dataDirs)
	args := append([]string{"generate"}, paths.GoDirs...)
	return sh.RunV("go", args...)
}

func goBindata(ctx context.Context) error {
	gbdbin := fp.Join(paths.RepoRoot, "bin/go-bindata")
	sl, err := fp.EvalSymlinks(gbdbin)
	if err == nil {
		_, err = os.Stat(sl)
	}
	if err == nil {
		fmt.Println("found go-bindata. assuming output is compatible. in the event of errors, build the vendored version and ensure it is first in $PATH.")
		return nil
	}
	gbdCmdPath := "github.com/jteeuwen/go-bindata/go-bindata"
	gbdSrc := fp.Join(paths.RepoRoot, "vendor", gbdCmdPath)
	var out []byte

	if _, err = os.Stat(fp.Join(gbdSrc)); err != nil {
		fmt.Println("go-bindata not vendored?!")
		return err
	}
	fmt.Println("building", gbdbin)
	//build, writing to dir we expect it to be in
	build := exec.CommandContext(ctx, "go", "build", "-o", gbdbin, gbdSrc)
	out, err = build.CombinedOutput()
	if err != nil {
		fmt.Printf("getting %s: error %s\noutput: %s", gbdCmdPath, err, out)
	}
	return err
}

func dataDirs(ctx context.Context) error {
	for _, d := range []string{
		"appliance",
		"disk",
		"qa",
	} {
		// Dirs must exist else go-bindata exits with error. Add files to the
		// dirs to override the defaults - search for uses of Asset() in pkgs.
		err := os.MkdirAll(fp.Join(paths.RepoRoot, "proprietary/data", d), 0755)
		if err != nil {
			fmt.Printf("creating data dir %s: %s", d, err)
			return err
		}
	}
	return nil
}

//build go code with desired flags
var build func(env map[string]string, args ...string) error

func init() {
	var args []string
	for _, a := range []string{
		//trimpath gains additional functionality in go1.13, could be useful
		//https://github.com/golang/go/issues/16860
		//https://github.com/golang/go/issues/22382
		"build",
		"-asmflags", "all=-trimpath=${GOPATH}/src",
		"-gcflags", "all=-trimpath=${GOPATH}/src -dwarf=false",
		"-ldflags", "-X 'main.buildId=${BUILD_INFO}' -s -w",
	} {
		args = append(args, os.ExpandEnv(a))
	}
	build = RunWCmd(nil, "go", args...)
}

//sh.RunCmd modified to call RunWith
func RunWCmd(env map[string]string, cmd string, args ...string) func(env2 map[string]string, args ...string) error {
	return func(env2 map[string]string, args2 ...string) error {
		var cenv map[string]string
		if env == nil {
			cenv = env2
		} else {
			cenv = env
			if env2 != nil {
				for k, v := range env2 {
					cenv[k] = v
				}
			}
		}
		return sh.RunWith(cenv, cmd, append(args, args2...)...)
	}
}

//like build, but outputs to work dir. if GOOS==windows, adds .exe suffix
func buildeach(env map[string]string, tags []string, args ...string) error {
	var sfx string
	if env != nil && env["GOOS"] == "windows" {
		sfx = ".exe"
	}
	for k, v := range env {
		fmt.Printf("%s=%s\n", k, v)
	}
	for _, a := range args {
		var cmdArgs []string
		if len(tags) > 0 {
			//tags takes a _space_ separated list
			cmdArgs = []string{"-tags", strings.Join(tags, " ")}
		}
		out := fp.Join(paths.WorkDir, fp.Base(a)) + sfx
		cmdArgs = append(cmdArgs, "-o", out, a)
		err := build(env, cmdArgs...)
		if err != nil {
			return err
		}
	}
	return nil
}

func workdir() {
	//ignore errors
	_ = os.Mkdir(paths.WorkDir, 0755)
}

//extract lib and header for compiling/linking. Only for things linking go-udev.
func libudev(ctx context.Context) error {
	tball := fp.Join(paths.RepoRoot, "build/udev.txz")
	if _, err := os.Stat(tball); os.IsNotExist(err) {
		fmt.Println("skipping udev tarball - does not exist")
		return nil
	}
	mg.CtxDeps(ctx, workdir)
	return sh.Run("tar", "xJf", tball, "-C", paths.WorkDir)
}

//return paths to pkgs imported by given package.
func depDirs(ctx context.Context, pkg string, tags []string) ([]string, error) {
	taglist := strings.Join(tags, " ")
	list := exec.CommandContext(ctx, "go", "list", "-f", "{{range .Deps}}{{.}}\n{{end}}", "-tags", taglist, pkg)
	out, err := list.CombinedOutput()
	if err != nil {
		return nil, err
	}
	allDeps := strings.Split(string(out), "\n")
	//filter out system deps, transform into absolute paths
	deps := []string{}
	for _, l := range allDeps {
		l = strings.TrimSpace(l)
		for _, pfx := range []string{
			paths.ImportPath,
			"github.com/",
			"build/",
			"golang.org/",
		} {
			if strings.HasPrefix(l, pfx) {
				deps = append(deps, strings.Replace(l, paths.ImportPath, paths.RepoRoot, 1))
			}
		}
	}
	return deps, nil
}
