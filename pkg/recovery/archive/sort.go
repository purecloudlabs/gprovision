// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package archive

/* Sort update names by date, newest first.
 * Names must be in a particular format
 *
 * sort value: yymmdd.build (float)
 * prod. os rev . platform .yyyy-mm-dd.build.ext
 */

import (
	"sort"
	"strconv"
	"strings"
)

type update struct {
	name string
	date float64 //yymmdd.hhmm
}
type updates []update

func (s updates) Less(i, j int) bool { return s[i].date < s[j].date }
func (s updates) Len() int           { return len(s) }
func (s updates) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

/* decode image name for sorting
 * sort value: yymmdd.build (float) -- prefers date for sort, but in the event of a collision a higer build wins
 * prod.os&rev.platform.yyyy-mm-dd.build
 *   0    1       2         3       4
 */
func decode(name string) (u update) {
	u.name = name
	u.date = 0.0
	parts := strings.Split(name, ".")
	if (len(parts) != 6) || (len(parts[3]) != 10) {
		return
	}
	dateIn := strings.Split(parts[3], "-")
	if len(dateIn) != 3 {
		return
	}
	for _, s := range dateIn {
		if !allDigits(s) {
			return
		}
	}
	yyyy := dateIn[0]
	mm := dateIn[1]
	dd := dateIn[2]

	if (len(parts[4]) == 0) || (!allDigits(parts[4])) {
		return
	}
	build := "." + parts[4]

	var err error
	//                               2015 02 33 .1593
	u.date, err = strconv.ParseFloat(yyyy+mm+dd+build, 64)
	if err != nil {
		u.date = 0.0
	}
	return
}

func allDigits(s string) bool {
	for _, c := range s {
		if !strings.ContainsRune("0123456789", c) {
			return false
		}
	}
	return true
}

func sortUpdates(names []string, oldestFirst bool) (sorted []string) {
	var s updates
	for _, name := range names {
		s = append(s, decode(name))
	}
	if oldestFirst {
		sort.Sort(s)
	} else {
		// "normal" sort is actually reversed because we want the most recent (greatest) date first
		sort.Sort(sort.Reverse(s))
	}
	for _, u := range s {
		sorted = append(sorted, u.name)
	}
	return
}
