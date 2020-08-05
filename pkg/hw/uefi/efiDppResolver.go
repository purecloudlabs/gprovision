// Copyright (C) 2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package uefi

import (
	"io/ioutil"
	"os"
	fp "path/filepath"

	"github.com/purecloudlabs/gprovision/pkg/hw/block"
	"github.com/purecloudlabs/gprovision/pkg/log"

	"github.com/u-root/u-root/pkg/mount"
)

type EfiPathSegmentResolver interface {
	//Returns description, does not require cleanup
	String() string

	//Mount fs, etc. You must call Cleanup() eventually.
	Resolve(suggestedBasePath string) (string, error)

	//For devices, returns BlkInfo. Returns nil otherwise.
	BlockInfo() *block.BlkInfo

	//Unmount fs, free resources, etc
	Cleanup()
}

type HddResolver struct {
	block.BlkInfo
	mountPoint string
}

var _ EfiPathSegmentResolver = (*HddResolver)(nil)

func (r *HddResolver) String() string { return r.BlkInfo.Device }

func (r *HddResolver) Resolve(basePath string) (string, error) {
	if len(r.mountPoint) > 0 {
		return r.mountPoint, nil
	}
	var err error
	if len(basePath) == 0 {
		basePath, err = ioutil.TempDir("", "uefiPath")
		if err != nil {
			return "", err
		}
	} else {
		fi, err := os.Stat(basePath)
		if err != nil || !fi.IsDir() {
			err = os.RemoveAll(basePath)
			if err != nil {
				return "", err
			}
			err = os.MkdirAll(basePath, 0755)
			if err != nil {
				return "", err
			}
		}
	}
	err = mount.Mount(r.BlkInfo.Device, basePath, r.BlkInfo.FsType.String(), "", 0)
	if err == nil {
		r.mountPoint = basePath
	}
	return r.mountPoint, err
}
func (e *HddResolver) BlockInfo() *block.BlkInfo { return &e.BlkInfo }

func (r *HddResolver) Cleanup() {
	if len(r.mountPoint) > 0 {
		_ = mount.Unmount(r.mountPoint, false, true)
	}
	r.mountPoint = ""
}

type PathResolver string

var _ EfiPathSegmentResolver = (*PathResolver)(nil)

func (r *PathResolver) String() string { return string(*r) }

func (r *PathResolver) Resolve(basePath string) (string, error) {
	if len(basePath) == 0 {
		log.Logf("uefi.PathResolver: empty base path")
	}
	return fp.Join(basePath, string(*r)), nil
}

func (r *PathResolver) BlockInfo() *block.BlkInfo { return nil }

func (r *PathResolver) Cleanup() {}
