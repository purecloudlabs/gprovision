// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

// +build !release

package testlog

import (
	"bufio"
	"regexp"
	"strings"
)

//a function that returns true if 'in' should be included in entries compared
type LineFilterer func(in string) (match bool)

//filter passing only calls to Msgf()
func FilterMsg() LineFilterer { return FilterPfx("MSG:") }

//filter passing only calls to Logf()
func FilterLog() LineFilterer { return FilterPfx("LOG:") }

//filter passing only lines with given prefix (note MSG:/LOG: added by Msgf/Logf)
func FilterPfx(pfx string) LineFilterer {
	return func(in string) bool { return strings.HasPrefix(in, pfx) }
}

//filter passing only calls to Logf, with given prefix
func FilterLogPfx(pfx string) LineFilterer { return FilterPfx("LOG:" + pfx) }

//filter passing only calls to Msgf, with given prefix
func FilterMsgPfx(pfx string) LineFilterer { return FilterPfx("MSG:" + pfx) }

//filter with given regex
func FilterRe(re string) LineFilterer {
	rx, err := regexp.Compile(re)
	if err != nil {
		panic(err)
	}
	return func(in string) bool {
		return rx.MatchString(in)
	}
}

//combine two filters; both must accept input
func FilterAnd(f1, f2 LineFilterer) LineFilterer {
	return func(in string) bool {
		return f1(in) && f2(in)
	}
}

//combine two filters; either may accept input
func FilterOr(f1, f2 LineFilterer) LineFilterer {
	return func(in string) bool {
		return f1(in) || f2(in)
	}
}

//Filter buffered log using lf as test. Return matches. Buffer is left empty.
//Assumes each entry is a single line.
func (tlog *TstLog) Filter(lf LineFilterer) []string {
	tlog.mu.RLock()
	defer tlog.mu.RUnlock()
	if tlog.Buf == nil {
		tlog.t.Error("nil buffer")
		return nil
	}
	var lines []string
	scanner := bufio.NewScanner(tlog.Buf)
	for scanner.Scan() {
		if lf(scanner.Text()) {
			lines = append(lines, scanner.Text())
		}
	}
	return lines
}

// A function that trims/transforms log lines before comparison. Used
// to make 'golden text' smaller and/or easier to read. Passed to
// LinesMustMatchTrimmed().
type LineCleaner func(in string) string

//trim first n bytes of each line
func TrimToIdx(idx int) LineCleaner {
	return func(in string) string {
		if len(in) < idx {
			return ""
		}
		return in[idx:]
	}
}

//trims off seq and anything following it
func TrimFromSeq(seq string) LineCleaner {
	return func(in string) string {
		split := strings.Split(in, seq)
		if len(split) == 0 {
			return ""
		}
		return split[0]
	}
}

//combines cleaners, applying f2 to the output of f1
func TrimAnd(f1, f2 LineCleaner) LineCleaner {
	return func(in string) string { return f2(f1(in)) }
}
