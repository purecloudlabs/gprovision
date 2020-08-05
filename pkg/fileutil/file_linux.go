// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package fileutil

import (
	"fmt"
	"io"
	"os"
	"syscall"

	"github.com/purecloudlabs/gprovision/pkg/log"
)

// Copy a file. Assumes any dirs have already been created. Copies metadata.
func CopyFile(src, dest string, destFlags int) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	return copyFileI(src, dest, info, destFlags)
}

//like CopyFile; use when file has already been stat'd.
func copyFileI(src, dest string, info os.FileInfo, destFlags int) error {
	out, err := os.OpenFile(dest, os.O_RDWR|os.O_CREATE|os.O_TRUNC|destFlags, 0666)
	if err != nil {
		return err
	}
	defer out.Close()
	in, err := os.OpenFile(src, os.O_RDONLY, 0400)
	if err != nil {
		return err
	}
	defer in.Close()
	n, err := io.Copy(out, in)
	if err != nil {
		out.Close()
		os.Remove(dest)
		return err
	}
	if n < info.Size() {
		// Files will frequently be larger as they are active logs.
		// Don't print anything in that case.
		return fmt.Errorf("copied %d bytes, expected %d", n, info.Size())
	}
	err = out.Chmod(info.Mode())
	if err != nil {
		return err
	}
	sys := info.Sys().(*syscall.Stat_t)
	err = out.Chown(int(sys.Uid), int(sys.Gid))
	if err != nil {
		log.Logf("error %s setting uid/gid of %s\n", err, dest)
	}
	err = os.Chtimes(dest, info.ModTime(), info.ModTime())
	return err
}
