// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package fileutil

import (
	"fmt"
	"gprovision/pkg/id"
	"gprovision/pkg/log"
	"io/ioutil"
	"os"
	fp "path/filepath"
	"strings"
	"syscall"
	"time"
)

// Return free space for FS containing dir, or -1 in the event of an error
func FreeSpace(dir string) int64 {
	var fs syscall.Statfs_t
	err := syscall.Statfs(dir, &fs)
	if err != nil {
		log.Logf("Error %s finding device free space\n", err)
		return -1
	}
	return int64(fs.Bavail) * fs.Bsize
}

// Returns human-readable free space (in MB) for FS containing given dir.
func FreeSpaceM(dir string) string {
	fs := FreeSpace(dir)
	if fs < 0 {
		return "(unknown - error)"
	}
	return ToMegs(fs)
}

//Copy files in flist to destDir, stripping srcDir. flist must be absolute paths.  NOTE: may be slow to return, due to use of O_SYNC
func CopySomeFiles(srcDir, destDir string, flist []string) error {
	errs := 0
	for _, src := range flist {
		if !strings.HasPrefix(src, srcDir) {
			return fmt.Errorf("File %s is not in dir %s", src, srcDir)
		}
		rel := strings.TrimPrefix(src, srcDir)
		dest := fp.Join(destDir, rel)
		err := os.MkdirAll(fp.Dir(dest), 0777)
		if err != nil {
			log.Logf("Failed to create %s: %s", fp.Dir(dest), err)
			log.Logf("Skipping %s and attempting to continue...", src)
			errs++
			continue
		}
		err = CopyFile(src, dest, os.O_SYNC)
		if err != nil {
			log.Logf("Failed to copy %s: %s", src, err)
			log.Logf("Skipping %s and attempting to continue...", src)
			errs++
			continue
		}
	}
	if errs != 0 {
		return fmt.Errorf("Encountered %d error(s); one or more logs are truncated or incomplete", errs)
	}
	return nil
}

//RecursiveCopy walks tree rooted at src, copies dir 'src' to a subdir of dest.
//
//NOTE: may be slow to return, due to use of O_SYNC.
func RecursiveCopy(src, dest string) error {
	destDir := fp.Join(dest, fp.Base(src))
	if err := os.MkdirAll(destDir, 0777); err != nil {
		return err
	}
	var walker fp.WalkFunc = func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Logf("error %s walking %s, skipping dir\n", err, path)
			return fp.SkipDir
		}
		relPath := strings.TrimPrefix(path, src)
		destPath := fp.Join(destDir, relPath)
		if info.IsDir() {
			err = os.Mkdir(destPath, 0777)
			if err != nil && !os.IsExist(err) {
				log.Logf("error %s creating %s\n", err, destPath)
				return err
			}
			sys := info.Sys().(*syscall.Stat_t)
			if err = os.Chown(destPath, int(sys.Uid), int(sys.Gid)); err != nil {
				log.Logf("chown: %s", err)
			}
			return nil
		}
		err = copyFileI(path, destPath, info, os.O_SYNC)
		if err != nil {
			log.Logf("error %s copying %s to %s\n", err, path, destPath)
		}
		return err
	}
	err := fp.Walk(src, walker)
	if err != nil {
		log.Msgf("error %s copying logs for %s\n", err, fp.Base(src))
	}
	return err
}

// Checks that a dir's inode does not change; if it does, call action().
// Call from goroutine. Returns (exiting goroutine) if action returns error.
func CheckDirPeriodic(dir string, delay time.Duration, action func() error) {
	var sys *syscall.Stat_t
	var inode uint64
	if delay == 0 {
		log.Logf("CheckDir: delay is 0, returning.\n")
		return
	}
	fi, err := os.Lstat(dir)
	if err == nil {
		sys = fi.Sys().(*syscall.Stat_t)
	}
	if sys != nil {
		inode = sys.Ino
	}
	for {
		time.Sleep(delay)
		fi, err = os.Lstat(dir)
		if err == nil {
			sys = fi.Sys().(*syscall.Stat_t)
		} else {
			sys = nil
		}
		if sys == nil {
			continue
		}
		if inode == sys.Ino {
			continue
		}
		log.Logf("CheckDir: dir has been moved (inode %d -> %d)\n", inode, sys.Ino)
		inode = sys.Ino
		err = action()
		if err != nil {
			return
		}
	}
}

//WaitForDir waits for a dir to exist, and, if watchedIsMountpoint is true,
//waits for a fs to mount there.
func WaitForDir(watchedIsMountpoint bool, watchDir string) {
	first := true
	sleep := func() {
		if first {
			log.Logf("WaitForDir: sleeping until %s exists\n", watchDir)
			first = false
		}
		time.Sleep(time.Second)
	}
	for {
		dir, err := fp.EvalSymlinks(watchDir)
		if os.IsNotExist(err) {
			sleep()
			continue
		}
		if err != nil {
			log.Fatalf(fmt.Sprintf("error %s", err))
		}
		_, err = os.Stat(dir)
		if err == nil || !os.IsNotExist(err) {
			if !watchedIsMountpoint || IsMountpoint(dir) {
				break
			}
		}
		sleep()
	}
	log.Logf("WatchDir exists now, setting up notifications...")
}

//IsMountpoint searchs for given dir in /proc/self/mountinfo, returns true if found
func IsMountpoint(dir string) bool {
	mi, err := ioutil.ReadFile("/proc/self/mountinfo")
	if err != nil {
		log.Logf("error %s", err)
		return false
	}
	for _, line := range strings.Split(string(mi), "\n") {
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		mp := mpFromLine(line)
		if mp == dir {
			return true
		}
	}
	return false
}

//used by IsMountpoint
func mpFromLine(line string) string {
	elements := strings.Split(line, " ")
	if len(elements) < 6 {
		//elements towards end of line can vary, but those towards beginning
		//seem to stay the same
		log.Logf("failed to parse mountinfo line, skipping: %s", line)
		return ""
	}
	return elements[4]
}

// Create dir in root with given owner, group, and mode
func MkdirOwned(root, dir, owner, group string, mode os.FileMode) bool {
	absDir := fp.Join(root, dir)
	err := os.Mkdir(absDir, mode) //mode will be ineffective, though it does need another arg
	if err != nil {
		log.Logf("failed to create %s: %s", dir, err)
		return false
	}
	uid, err := id.GetUID(root, owner)
	if err != nil {
		log.Logln(err)
	}
	gid, err := id.GetGID(root, group)
	if err != nil {
		log.Logln(err)
	}
	if uid < 0 || gid < 0 {
		//leave the dir, in case we can limp along with incorrect perms
		log.Logf("MkdirOwned(%s, %s, %s, %d): failed to set owner %d/group %d", dir, owner, group, mode, uid, gid)
		return false
	}
	err = os.Chown(absDir, uid, gid)
	if err == nil {
		err = os.Chmod(absDir, mode) //mode must be set last; changing uid/gid will unset special bits
	}
	if err != nil {
		log.Logf("MkdirOwned(%s, %s, %s, %s, %d): err %s", root, dir, owner, group, mode, err)
		return false
	}
	return true
}

/*
ForcePathCase ensures correct case for some dir or file target in basepath.
If target exists with different case, rename.
If multiple variations exist, delete all but one.
*/
func ForcePathCase(basepath, target string) (success bool) {
	lowerTarget := strings.ToLower(target)
	content, err := ioutil.ReadDir(basepath)
	if err != nil {
		log.Logf("ForcePathCase(%s,%s): err %s", basepath, target, err)
		return
	}
	haveCorrectCase := false
	var caseInsMatches []string //case-insensitive matches; if 'Image' exists, it won't be added
	for _, item := range content {
		if item.Name() == target {
			haveCorrectCase = true
			continue
		}
		if strings.ToLower(item.Name()) == lowerTarget {
			caseInsMatches = append(caseInsMatches, item.Name())
		}
	}
	if len(caseInsMatches) == 0 && haveCorrectCase {
		//only one, with correct case. no action necessary
		success = true
		return
	}
	if len(caseInsMatches) == 0 /* && !haveCorrectCase */ {
		contents := ""
		for _, i := range content {
			contents += fmt.Sprintf("%s ", i.Name())
		}
		log.Logf("ForcePathCase(%s,%s): no candidate directories?!\n%s\n", basepath, target, contents)
		return
	}
	for i, m := range caseInsMatches {
		current := fp.Join(basepath, m)
		if i == 0 {
			//rename
			new := fp.Join(basepath, target)
			err = os.Rename(current, new)
			if err != nil {
				log.Logf("ForcePathCase(%s,%s): err %s renaming %s to %s", basepath, target, err, current, new)
			}
		} else {
			//delete
			err = os.RemoveAll(current)
			if err == nil {
				log.Logf("ForcePathCase(%s,%s): removed dir %s with bad case", basepath, target, m)
			} else {
				log.Logf("ForcePathCase(%s,%s): err %s removing dir %s with bad case", basepath, target, err, m)
			}
		}
	}
	success = true
	return
}
