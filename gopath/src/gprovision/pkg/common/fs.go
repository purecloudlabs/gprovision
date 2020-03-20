// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package common

type Pather interface {
	Path() string
}
type PatherMock string

func (p *PatherMock) Path() string { return string(*p) }

type Mounter interface {
	Mount()
	IsMounted() bool
	SetMountpoint(path string)
	Umount()
}
type Formatter interface {
	Format(label string) error
}

type Fstaber interface {
	WriteFstab(uid, gid string, parts ...FS)
	FstabEntry(uid, gid string) (entry string)
	FstabMountpoint() string
}

type FS interface {
	Pather
	Mounter
	Formatter
	Fstaber
	SetOwnerAndPerms(uid, gid string)
}
