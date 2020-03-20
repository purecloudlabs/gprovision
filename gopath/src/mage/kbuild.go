// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

// +build mage

package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	fp "path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
	"github.com/magefile/mage/target"
)

// Kernel builds.
// All kernel builds share a build dir to save time; different kernel
// configs can safely be built in one build dir and will reuse .o's - as long
// as they are not built at the same time.
type Kernel mg.Namespace

// This mutex protects against multiple kernel builds happening at once.
//
// As a side effect, we cannot rely on any files in the build dir after unlocking
// the mutex.
var KbuildMtx sync.Mutex

//kernel for testing
func (Kernel) Noinitramfs(ctx context.Context) error {
	mg.CtxDeps(ctx, paths)
	ki, err := FindKernel()
	if err != nil {
		return err
	}

	ki.Dest = KNoInitramfs
	ki.Opts.Localversion = "-vm_test"
	ki.Opts.Compression = NoCompression
	return ki.BuildKernel()
}

//"normal" kernel
func (Kernel) Boot(ctx context.Context) error {
	mg.CtxDeps(ctx, Initramfs.Boot)

	ki, err := FindKernel()
	if err != nil {
		return err
	}

	ki.Dest = KBoot
	ki.Opts.Localversion = "-" + strings.Split(KBootName, ".")[0]
	ki.Opts.Compression = Xz
	ki.Opts.Initramfs = InitramfsBoot
	return ki.BuildKernel()
}

//provisioning kernel for pxeboot
func (Kernel) Linuxmfg(ctx context.Context) error {
	mg.CtxDeps(ctx, Initramfs.Mfg)

	ki, err := FindKernel()
	if err != nil {
		return err
	}

	ki.Dest = KMfg
	ki.Opts.Localversion = "-" + strings.Split(MfgName, ".")[0]
	ki.Opts.Compression = Xz
	ki.Opts.Initramfs = InitramfsMfg
	return ki.BuildKernel()
}

type KInfo struct {
	BuildNum   uint64
	ConfigPath string
	SrcDir     string
	BuildDir   string
	Opts       KOpts
	Dest       string
}
type KOpts struct {
	Initramfs    string
	Localversion string
	Compression  CompMode
}
type CompMode int

const (
	NoCompression CompMode = iota
	Xz
)

func (ki KInfo) BuildKernel() error {
	//are dependencies newer?
	//warning - assumes kernel source will never be modified
	srcs := []string{ki.ConfigPath}
	if len(ki.Opts.Initramfs) > 0 {
		//also depend on initramfs if it exists
		srcs = append(srcs, ki.Opts.Initramfs)
	}
	changed, err := target.Path(ki.Dest, srcs...)
	if err != nil {
		return err
	}
	if !changed {
		fmt.Printf("skipping build of '%s' kernel...\n", strings.TrimPrefix(ki.Opts.Localversion, "-"))
		return nil
	}
	fmt.Printf("build %s...\n", ki.Opts.Localversion)
	KbuildMtx.Lock()
	defer KbuildMtx.Unlock()

	err = os.Remove(ki.Dest)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	cfg, err := ioutil.ReadFile(ki.ConfigPath)
	if err != nil {
		return err
	}
	//kernel source always contains this file
	_, err = os.Stat(fp.Join(ki.SrcDir, "Kbuild"))
	if err != nil {
		return err
	}
	if err = os.MkdirAll(ki.BuildDir, 0755); err != nil {
		return err
	}
	//add branch or PR id if not master, so provenance is easy to determine
	if branch := os.Getenv("BRANCH_NAME"); branch != "master" {
		if branch == "" {
			branch = "UNKNOWN-BRANCH"
		}
		ki.Opts.Localversion += "_" + branch
	}
	patchedCfg, err := kOptSet(cfg, "CONFIG_LOCALVERSION", ki.Opts.Localversion, true)
	if err != nil {
		return err
	}
	patchedCfg, err = kOptSet(patchedCfg, "CONFIG_INITRAMFS_SOURCE", ki.Opts.Initramfs, true)
	if err != nil {
		return err
	}
	if len(ki.Opts.Initramfs) > 0 {
		patchedCfg, err = kOptSet(patchedCfg, "CONFIG_INITRAMFS_ROOT_UID", "0", false)
		if err != nil {
			return err
		}
		patchedCfg, err = kOptSet(patchedCfg, "CONFIG_INITRAMFS_ROOT_GID", "0", false)
		if err != nil {
			return err
		}
		patchedCfg, err = kOptSet(patchedCfg, "CONFIG_INITRAMFS_COMPRESSION", "", true)
		if err != nil {
			return err
		}
	}
	if ki.Opts.Compression == Xz {
		patchedCfg, err = kOptSet(patchedCfg, "CONFIG_KERNEL_XZ", "y", false)
	} else {
		patchedCfg, err = kOptUnset(patchedCfg, "CONFIG_KERNEL_XZ")
	}
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(fp.Join(ki.BuildDir, ".config"), patchedCfg, 0755)
	if err != nil {
		return err
	}
	fmt.Printf("config written\n")

	ki.writeVersion()
	fmt.Printf("version written\n")

	fmt.Printf("make ...\n")
	nproc := strconv.Itoa(runtime.NumCPU() + 1)
	err = sh.Run("make",
		"-C", ki.SrcDir,
		"O="+ki.BuildDir,
		"-j", nproc,
		"-l", nproc,
		"bzImage")
	if err != nil {
		return err
	}
	fmt.Println("make done")
	fmt.Printf("move to dest %s...\n", ki.Dest)
	kernel := fp.Join(ki.BuildDir, "arch/x86/boot/bzImage")
	return os.Rename(kernel, ki.Dest)
}

//write .version file, containing number seen in `uname -a`
func (ki KInfo) writeVersion() error {
	if ki.BuildNum == 0 {
		bn, ok := os.LookupEnv("BUILD_NUMBER")
		if ok && len(bn) > 0 {
			i, err := strconv.ParseUint(bn, 10, 64)
			if err != nil {
				return err
			}
			ki.BuildNum = i
		}
	}
	if ki.BuildNum > 0 {
		//written number must be one less than desired - it's incremened when make starts
		ki.BuildNum--
	}
	vers := fp.Join(ki.BuildDir, ".version")
	return ioutil.WriteFile(vers, []byte(fmt.Sprintf("%d", ki.BuildNum)), 0644)
}

//prevent multiple simultaneous download/upload attempts for kernel source
var dlMtx sync.Mutex

func FindKernel() (*KInfo, error) {
	matches, err := fp.Glob(fp.Join(RepoRoot, "linux-*.config"))
	if err != nil {
		return nil, err
	}
	if len(matches) != 1 {
		return nil, fmt.Errorf("want 1 glob match for linux config, got %d: %v", len(matches), matches)
	}
	cfg := matches[0]
	src := strings.TrimSuffix(cfg, ".config")
	var fi os.FileInfo
	dlMtx.Lock()
	defer dlMtx.Unlock()
	if fi, err = os.Stat(src); err != nil {
		err = getKSrc(src)
		if err != nil {
			return nil, err
		}
	} else if !fi.IsDir() {
		return nil, fmt.Errorf("src dir %s is not a dir", src)
	}
	build := KBuild
	err = os.MkdirAll(build, 0755)
	if err != nil {
		return nil, err
	}
	return &KInfo{
		SrcDir:     src,
		ConfigPath: cfg,
		BuildDir:   build,
	}, nil
}

// Set an opt in kernel config. Opt must exist, either commented out or with a
// value. Otherwise, return an error - if the opt isn't visible it means the
// opt depends on something else that isn't enabled.
//
// If val is a string, either set isStr or include surrounding quotes.
func kOptSet(cfg []byte, key, val string, isStr bool) ([]byte, error) {
	if isStr {
		if val == "" {
			val = `""`
		} else {
			if !strings.HasPrefix(val, `"`) {
				val = `"` + val
			}
			if !strings.HasSuffix(val, `"`) {
				val = val + `"`
			}
		}
	}
	cpy := make([]byte, len(cfg))
	copy(cpy, cfg)
	unset := []byte(fmt.Sprintf("\n# %s is not set", key))
	set := []byte(fmt.Sprintf("\n%s=", key))
	newval := []byte(fmt.Sprintf("%s=%s", key, val))
	if bytes.Contains(cfg, unset) {
		return bytes.Replace(cpy, unset, newval, 1), nil
	}
	if i := bytes.Index(cpy, set); i > -1 {
		i += 1 //push past the newline
		//already set to some value; replace to end of line
		if l := bytes.Index(cpy[i:], []byte("\n")); l > -1 {
			return bytes.Replace(cpy, cpy[i:i+l], newval, 1), nil
		}
	}
	return nil, fmt.Errorf("failed to find location of %s in config", key)
}

// Like kOptSet, but unsets an item
func kOptUnset(cfg []byte, key string) ([]byte, error) {
	unset := []byte(fmt.Sprintf("\n# %s is not set", key))
	set := []byte(fmt.Sprintf("\n%s=", key))
	newval := []byte(fmt.Sprintf("# %s is not set", key))
	if bytes.Contains(cfg, unset) {
		return cfg, nil
	}
	cpy := make([]byte, len(cfg))
	copy(cpy, cfg)
	if i := bytes.Index(cpy, set); i > -1 {
		i += 1 //push past the newline
		//already set to some value; replace to end of line
		if l := bytes.Index(cpy[i:], []byte("\n")); l > -1 {
			return bytes.Replace(cpy, cpy[i:i+l], newval, 1), nil
		}
	}
	return nil, fmt.Errorf("failed to find location of %s in config", key)
}

//determines version from path, downloads from blobstore
//if blobstore is absent, dl from kernel.org and upload to blobstore
//once downloaded, extract to path
func getKSrc(path string) error {
	ver := fp.Base(path)
	fname := ver + ".tar.xz"
	tballPath := fp.Join(RepoRoot, fname)
	blobpath := BlobstoreKDir + fname

	err := blobcp(blobpath, tballPath, false)
	if err != nil {
		fmt.Printf("%s not present in blobstore, trying kernel.org...", fname)
		tb, err := os.Create(tballPath)
		if err != nil {
			fmt.Printf("unable to create %s: %s", tballPath, err)
			return err
		}
		//get kernel from kernel.org
		korgUrl := KOrgKDir + fname
		resp, err := http.Get(korgUrl)
		if err != nil {
			tb.Close()
			fmt.Printf("unable to download %s: %s", korgUrl, err)
			return err
		}
		_, err = io.Copy(tb, resp.Body)
		tb.Close()
		resp.Body.Close()
		if err != nil {
			fmt.Printf("writing %s: %s", tballPath, err)
			return err
		}
		//do not delete old versions, let s3 expire them

		//upload new
		err = blobcp(tballPath, blobpath, true)
		if err != nil {
			fmt.Printf("unable to upload kernel to %s: %s", blobpath, err)
			return err
		}
	}
	//extract
	err = sh.Run("tar", "xJf", tballPath, "-C", RepoRoot)
	if err != nil {
		fmt.Printf("extracting tarball: %s", err)
	}
	return err
}
