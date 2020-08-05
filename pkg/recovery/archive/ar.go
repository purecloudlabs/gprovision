// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

//Package archive sorts, verifies, and extracts .upd (xz-compressed tar) files.
//Xz defaults to crc64 checksums but can support others such as sha256.
//.upd files *must* use sha256 or they will be considered invalid.
package archive

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/purecloudlabs/gprovision/pkg/common/strs"
	dt "github.com/purecloudlabs/gprovision/pkg/disktag"
	futil "github.com/purecloudlabs/gprovision/pkg/fileutil"
	"github.com/purecloudlabs/gprovision/pkg/log"
	"github.com/purecloudlabs/gprovision/pkg/recovery/archive/meta"
	"github.com/purecloudlabs/gprovision/pkg/recovery/disk"
	"github.com/purecloudlabs/gprovision/pkg/recovery/history"
)

var decompressBuf = "/buf"

//validate an update file
//if !lowMem, extract to decompressBuf
func validateExtractUpd(updPath string) (err error) {
	var buf *os.File
	if lowMemoryDevice {
		log.Logf("device has low memory, falling back to 2-step process")
		buf, err = os.OpenFile("/dev/null", syscall.O_WRONLY, 0600)
	} else {
		buf, err = os.Create(decompressBuf)
		if err != nil {
			return
		}
	}
	defer buf.Close()
	updName := path.Base(updPath)
	log.Msg("checking " + updName[:len(updName)-4])

	if !futil.IsXZSha256(updPath) {
		err = fmt.Errorf("%s: wrong checksum", updName)
		log.Logf("%s", err)
		return
	}

	xz := exec.Command("xz", "-dc", updPath)
	xz.Stdout = buf
	errbuf := new(bytes.Buffer)
	xz.Stderr = errbuf

	xzDone := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		futil.ShowProgress(xzDone, "Validating", decompressBuf)
		wg.Done()
	}()

	err = xz.Run()
	close(xzDone)
	wg.Wait()
	if err != nil {
		/*
			did we run out of space due to low mem? if so, try again
			shouldn't get here - lowMemoryDevice should already be correct.
			however, factory restore/buildroot changes could potentially affect this
		*/
		//if device runs out of memory, we might see "no space left..."
		//or, xz might just get killed

		space := futil.FreeSpace("/")
		buf.Close()
		os.Remove(decompressBuf)
		log.Logf("xz exited with error; free space (bytes): %d", space)
		stderr := errbuf.String()
		if !lowMemoryDevice {
			noSpaceStr := strings.Contains(stderr, "No space left on device")
			if noSpaceStr || isExitWithSig(err) || space < 10*1024*1024 {
				log.Logf("xz appears to have run out of space, retrying with lowMemoryDevice=true")
				log.Logf("xz err=%s; stderr=%s", err, stderr)

				lowMemoryDevice = true
				return validateExtractUpd(updPath)
			}
		}
		log.Logf("error during decompression of %s: %s\nstderr=%s", updPath, err, stderr)
		//try another, if it exists
		return err
	}
	if lowMemoryDevice {
		log.Logf("decompressed valid update %s", updPath)
	} else {
		var fi os.FileInfo
		fi, err = os.Stat(decompressBuf)
		if err != nil {
			log.Logf("stat error during decompression of %s: %s\n", updPath, err)
			return
		}
		updSize := fi.Size() / (1024 * 1024)

		log.Logf("decompressed valid %dM update %s", updSize, updPath)
	}
	return
}

//does the error come from a process terminated by a signal?
func isExitWithSig(execErr error) bool {
	ee, ok := execErr.(*exec.ExitError)
	if !ok {
		return false
	}
	ws, ok := ee.Sys().(syscall.WaitStatus)
	if !ok {
		return false
	}
	return ws.Signaled()
}

//extract data to target
func applyUpd(target *disk.Filesystem) bool {
	/* use gnu tar rather than busybox-tar or bsdtar to ensure
	 * permissions, owner, extended attr's, etc are retained
	 *
	 * need --xattrs-include=... ??
	 *
	 * to be able to extract concatenated archives, need -i option - not
	 * available in busybox tar or bsdtar
	 * xz supports concatenation, so no problem there
	 * not sure if concat order will make a difference or not
	 *
	 * suppress timestamp warnings, seen with centos tarball. voluminous
	 * stderr caused problems for goroutine reading output. two-pronged
	 * solution: suppress warnings, use rescue() to prevent panics
	 */

	untar := exec.Command("tar", "x", "-i", "--xattrs", "--totals=USR1", "--warning=no-timestamp", "-C", target.Path())
	if !lowMemoryDevice {
		f, err := os.Open(decompressBuf)
		if err != nil {
			log.Logf("can't open decompress buffer: %s - trying original file", err)
			lowMemoryDevice = true //just read from original file
		} else {
			defer f.Close()
			defer os.Remove(decompressBuf)
			untar.Stdin = f
		}
	}
	if lowMemoryDevice {
		//pipe from xz
		xz := exec.Command("xz", "-dc", updateFullPath)
		log.Logln(xz.Args)
		var err error
		untar.Stdin, err = xz.StdoutPipe()
		if err != nil {
			log.Logf("error executing xz: %s", err)
		}
		if err := xz.Start(); err != nil {
			log.Logf("start xz: %s", err)
		}
		defer func() {
			err := xz.Wait()
			if err != nil {
				log.Logf("error executing xz: %s", err)
			}
		}()
	}

	log.Logln(untar.Args)

	untarDone := make(chan struct{})
	defer close(untarDone)
	bgUnTarStatus(untarDone, untar) //spawns a goroutine to update status in the background

	out, err := untar.Output()
	if err != nil {
		log.Logf("untar error %s, output\n%s", err, out)
		return false
	}
	return true
}

//print tar progress to lcd
//tried --checkpoint-action=echo="%T", but that's only supported
//in very new tar's. --totals=SIG is much older
func bgUnTarStatus(done chan struct{}, untar *exec.Cmd) {
	out, pserr, err := os.Pipe()
	if err != nil {
		log.Logf("Can't create untar pipe: %s", err)
		return
	}
	untar.Stderr = pserr
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Logf("unTarStatus recovered: %#v", r)
			}
		}()
		defer pserr.Close()
		defer out.Close()
		buf := make([]byte, 128)
		i := 0
		for {
			time.Sleep(1 * time.Second)
			select {
			case <-done:
				log.Logf("unTarStatus: done, %ds", i)
				return
			default:
				i++
			}
			if err := untar.Process.Signal(syscall.SIGUSR1); err != nil {
				log.Logf("signal tar: %s", err)
			}
			time.Sleep(50 * time.Millisecond) //give it time to respond
			blen, _ := out.Read(buf)
			var line []byte
			if blen > 0 {
				/* Total bytes read: 7924664320 (7.4GiB, 95MiB/s) */
				line = bytes.TrimSpace(buf)
			}
			if len(line) > 0 {
				nl := bytes.IndexRune(line, '\n')
				if nl > -1 {
					line = line[:nl]
				}
				//ensure we don't print out some error message containing ( ... ,
				if bytes.Contains(line, []byte("Total bytes")) {
					lparen := bytes.IndexRune(line, '(') + 1
					rparen := bytes.IndexRune(line[lparen:], ',')
					if lparen > 0 && rparen > 2 {
						log.Msgf("Writing... %s", line[lparen:rparen+lparen-2])
					} else {
						log.Logf("untar stderr: %s", bytes.Replace(buf, []byte{0}, nil, -1))
					}
				} else {
					log.Logf("untar stderr: %s", bytes.Replace(buf, []byte{0}, nil, -1))
				}
			} else {
				log.Logf("untar - no output")
			}
		}
	}()
}

//find updates, return a list of them in order of preference (newest first)
func listUpdates(updPath string, oldestFirst bool) []string {
	entries, err := ioutil.ReadDir(updPath)
	if err != nil {
		return nil
	}
	var unsorted []string
	for _, item := range entries {
		if item.IsDir() {
			continue
		}
		fname := item.Name()
		if !strings.HasSuffix(fname, ".upd") {
			log.Logf("listUpdates: skipping %s due to suffix", fname)
			continue
		}
		if !strings.HasPrefix(fname, strs.ImgPrefix()) {
			log.Logf("listUpdates: skipping %s due to prefix", fname)
			continue
		}
		fpath := path.Join(updPath, fname)
		if futil.IsXZSha256(fpath) {
			unsorted = append(unsorted, fpath)
		} else {
			log.Logf("listUpdates: skipping %s due to bad signature", fname)
		}
	}
	return sortUpdates(unsorted, oldestFirst)
}

func trimmedName(upd string) string {
	upd = path.Base(upd)
	upd = strings.TrimSuffix(upd, ".upd")
	upd = strings.TrimPrefix(upd, strs.EmergPfx())
	return strings.TrimPrefix(upd, "_")
}

var updateFullPath string //full path to valid update. only used for two-step, low-mem process
var lowMemoryDevice bool  //see lowMem below

//searches for a valid update. if emergencyImage != "", only consider it. otherwise considers any in given dir.
//if lowmem is true, xz's output is thrown away for the validation round and the file is decompressed again for piping to tar
func FindValidUpd(emergencyImage, imgopt, dir string, lowMem bool) (valid, userCancel bool) {
	lowMemoryDevice = lowMem
	var choices []string
	history.Load()
	if emergencyImage != "" {
		choices = []string{emergencyImage}
	} else {
		oldestFirst := false
		if imgopt == "" {
			//check the env var, which could be set through the grub menu
			imgopt = strings.Trim(strings.ToLower(os.Getenv("PREFERRED_UPDATE")), "-_")
		}
		if imgopt == "oldest" {
			log.Msg("sorting oldest/original image first")
			oldestFirst = true
		}
		choices = listUpdates(dir, oldestFirst)
		if imgopt == "menu" {
			choices = Menu(choices)
			if choices == nil {
				return false, true
			}
		}
	}
	return findValidUpd(choices), false
}

func findValidUpd(choices []string) bool {
	for idx, upd := range choices {
		trimmed := trimmedName(upd)
		imgMeta, err := meta.Read(upd)
		if err == nil {
			if imgMeta.ImgName == trimmed {
				log.Logf("%s: metadata matches file name", trimmed)
			} else {
				//don't do anything, just log
				log.Logf("%s: metadata disagrees on name\n%s", upd, imgMeta)
			}
		}
		dtag := dt.ImgToDTag(trimmed)
		ok := history.Check(dtag)
		if !ok {
			if idx == len(choices)-1 {
				//last item
				log.Logf("history check failed for last or only image %s, applying anyway", upd)
			} else {
				log.Msgf("history check failed for choice %d (%s)", idx, dtag)
				continue
			}
		}

		err = validateExtractUpd(upd)
		if err == nil {
			dt.Set(dtag)
			updateFullPath = upd
			log.Msg("valid: " + trimmed) //log the real name, not the disktag name
			return true
		} else {
			if len(choices) > 1 {
				log.Msg("invalid update. next...")
			} else {
				log.Msg("checks failed on only image")
				time.Sleep(5 * time.Second)
			}
		}
	}
	return false
}

func ApplyUpdate(target *disk.Filesystem) {
	if !applyUpd(target) {
		log.Fatalf("failed to apply validated update")
	}
}
