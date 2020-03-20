// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package erase

import (
	"bufio"
	"bytes"
	"fmt"
	"gprovision/pkg/common"
	"gprovision/pkg/common/strs"
	"gprovision/pkg/hw/cfa"
	"gprovision/pkg/hw/power"
	"gprovision/pkg/log"
	"gprovision/pkg/recovery/disk"
	"io/ioutil"
	"os"
	fp "path/filepath"
	"strings"
	"syscall"
	"time"
)

const (
	bootFile            = "data_erase.boot"
	unrecoverableErrVal = "UNRECOVERABLE_ERROR"
)

//call after all disks have been successfully erased.
//remove/modify files as necessary to prevent reboot into erase mode
func success(recov common.FS) {
	if recov.IsMounted() {
		f := fp.Join(recov.Path(), bootFile)
		os.Remove(f)
	}
	syscall.Sync()
	//can't use log.Preboot because that disables the lcd
	log.Finalize()
	disk.UnmountAll(true)
	msg := "Data erase completed successfully. It is safe to unplug this unit now."
	log.Log(msg)
	_ = cfa.DefaultLcd.BlinkMsg(msg, cfa.Fade, 2*time.Second, 48*time.Hour)
	power.RebootSuccess()
}

//use LCD to communicate that Data Erase failed (i.e. a drive did not come up)
//if write is true, patch the boot file to immediately jump here
func unrecoverableFailure(recov common.Pather, write bool) {
	if write && recov != nil {
		f := fp.Join(recov.Path(), bootFile)
		go writeErrToBootFile(f)
	}
	msg := "Data Erase: unrecoverable failure. Data MAY remain. Contact customer support to discuss options."
	_ = cfa.DefaultLcd.BlinkMsg(msg, cfa.Flash, 2*time.Second, 48*time.Hour)
	log.Fatalf("unrecoverable failure")
}

//modify bootFile to change kernel boot args
func writeErrToBootFile(f string) {
	for {
		buf, err := ioutil.ReadFile(f)
		if err != nil {
			continue
		}
		var out bytes.Buffer
		scanner := bufio.NewScanner(bytes.NewReader(buf))
		for scanner.Scan() {
			l := scanner.Text()
			if strings.HasPrefix(strings.TrimSpace(l), "kernel") {
				append := " " + strs.EraseEnv() + "=" + unrecoverableErrVal
				l += append
			}
			out.WriteString(l)
			out.WriteRune('\n')
		}
		err = scanner.Err()
		if err != nil {
			continue
		}
		err = ioutil.WriteFile(f, out.Bytes(), 0644)
		if err != nil {
			continue
		}
	}
}

/* put message on screen periodically
use:
	eraseDone = make(chan struct{})
	defer close(eraseDone)
	go decompressStatus(eraseDone, est, spinner)

*/
func tmax(a, b time.Duration) time.Duration {
	if a < b {
		return b
	}
	return a
}

//return s, truncated after first occurrence of r or at len==max, whichever is first
func truncate(s string, r rune, max int) string {
	i := strings.IndexRune(s, r) + 1
	if i == 0 {
		i = len(s)
	}
	if i > max {
		i = max
	}
	return s[:i]
}

func eraseStatus(times chan time.Duration, est time.Duration, s *cfa.Spinner) {
	start := time.Now()
	finishEst := start.Add(est)
	var estimate time.Duration
	counter := 0
	const incr = 3
	spin := true
	for {
		time.Sleep(incr * time.Second)
		select {
		case val, more := <-times:
			if !more {
				return
			}
			estimate = tmax(estimate, val)
			finishEst = start.Add(estimate)
		default:
		}
		//at first, rewrite each time. after that, rewrite once per minute
		//this is to get a good estimate onto the display within reasonable time
		if (counter < 30/incr) || (counter%(60/incr) == 0) {
			now := time.Now().Add(-1 * time.Minute) //better to overshoot than undershoot
			if spin && now.Before(finishEst) {
				est := finishEst.Sub(now).String()
				s.Msg = fmt.Sprintf("Erasing... %s (est) to go", truncate(est, 'm', 6))
				_ = s.Display()
			} else {
				spin = false
				log.Msgf("Erasing... elapsed time %s", now.Sub(start))
			}
		} else if spin {
			s.Next()
		}
		counter++
	}
}

//find 'seq' in 'buf', return slice containing seq and delimited by newlines
func getLine(buf []byte, seq string) (b []byte) {
	idx := bytes.Index(buf, []byte(seq))
	if idx == -1 {
		return
	}
	end := bytes.IndexByte(buf[idx:], '\n')
	if end == -1 {
		end = len(buf)
	} else {
		end += idx
	}
	begin := bytes.LastIndexAny(buf[:idx], "\n")
	if begin == -1 {
		begin = 0
	}
	return buf[begin:end]
}

/* get secure erase time estimate - normal or enhanced
 * if time in question is missing or doesn't start with
 * "XXXmin", falls back to 200 min (approximately what
 * 1TB Seagate ST91000640NS reports)
 */
func getSEtime(info []byte, enhanced bool) (t time.Duration) {
	t = 200 * time.Minute
	line := string(getLine(info, "SECURITY ERASE UNIT"))
	times := strings.Split(line, ".")
	nrTimes := len(times)
	if (nrTimes < 1) || (nrTimes > 3) || (len(times[2]) > 1) {
		//line format not as expected
		log.Logf("getTime: problem parsing %q / %s", times, line)
		return
	}
	enh0 := strings.Contains(times[0], "ENHANCED")
	enh1 := strings.Contains(times[1], "ENHANCED")
	var choice string
	switch enhanced {
	case true:
		if enh0 {
			choice = times[0]
		} else if enh1 {
			choice = times[1]
		}
	case false:
		if !enh0 {
			choice = times[0]
		} else if !enh1 {
			choice = times[1]
		}
	}
	choice = strings.TrimSpace(choice)
	m := strings.Index(choice, "min")
	if m > 0 {
		choice = choice[:m+1]
	}
	d, err := time.ParseDuration(choice)
	if err == nil {
		t = d
	} else {
		log.Logf("getSEtime: error %s for %s", err, choice)
	}
	return
}
