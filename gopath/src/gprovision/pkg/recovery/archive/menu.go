// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package archive

import (
	"fmt"
	"gprovision/pkg/common/strs"
	"gprovision/pkg/hw/cfa"
	"gprovision/pkg/log"
	fp "path/filepath"
	"strings"
	"time"
)

//display a menu. if lcd is present, uses that; otherwise vga.
func Menu(choices []string) []string {
	var choice cfa.Choice
	if cfa.DefaultLcd == nil {
		return VgaMenu(choices)
	}
	var shorts []string
	for _, c := range choices {
		s := fp.Base(c)
		prefixes := strings.SplitAfter(strs.ImgPrefix(), ".")
		for _, p := range prefixes {
			s = strings.TrimPrefix(s, p)
		}
		s = strings.TrimSuffix(s, ".upd")
		shorts = append(shorts, s)
	}
	choice, _ = cfa.DefaultLcd.MenuWithConfirm("image menu", cfa.Strs2LTxt(shorts...), 5*time.Minute, time.Minute, false)
	if choice >= 0 {
		return []string{choices[choice]}
	}
	return nil
}

//list images on screen, ask user to make a choice
//returns string array with length 1
func VgaMenu(choices []string) []string {
	log.Msg("displaying menu on vga")
	fmt.Printf("\n\n=======================================\n")
	fmt.Println("Images available:")
	for i, c := range choices {
		fmt.Printf("\t%d. %s\n", i+1, c)
	}
	fmt.Println("To cancel, press reset button")
	item := 0
	for item < 1 || item > len(choices) {
		fmt.Printf("Enter a number, 1-%d inclusive: ", len(choices))
		fmt.Scanln(&item)
	}
	choice := choices[item-1]
	log.Logf("menu choice: %s", choice)
	return []string{choice}
}
