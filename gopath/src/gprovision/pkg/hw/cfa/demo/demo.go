// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

// +build !release

//Demo app to exercise a compatible Crystalfontz LCD.
package main

import (
	"flag"
	"fmt"
	"gprovision/pkg/hw/cfa"
	"gprovision/pkg/hw/cfa/serial"
	"os"
	"runtime/pprof"
	"runtime/trace"
	"time"
)

const longMsg = "Data Erase: unrecoverable failure. Data MAY remain. Contact customer support to discuss options."

func main() {
	basics := flag.Bool("basics", false, "test basics")
	menu := flag.Int("menu", 0, "display n-element menu")
	events := flag.Bool("e", false, "print key events")
	cursor := flag.Bool("c", false, "cycle cursor style")
	ping := flag.Bool("p", false, "ping lcd rapidly. ctrl-c to exit")
	out := flag.String("out", "", "output serial activity to this file")
	backlight := flag.Bool("backlight", false, "cycle backlight intensity")
	spin := flag.Bool("spin", false, "display spinner for 10s")
	fade := flag.Bool("fade", false, "display message that fades in and out, for 10s")
	multi := flag.Bool("multi", false, "test multiple functions")
	long := flag.Bool("long", false, "display long message")
	msg := flag.String("msg", longMsg, "message to display (-fade/-multi/-long only)")
	pressAny := flag.Bool("press", false, "test  'press any key to interrupt...'")
	boot := flag.Bool("boot", false, "demo of what boot menu could look like")
	dbg := flag.String("dbg", "0000", "bits to set debug flags, '?' for help")
	question := flag.Bool("q", false, "ask question")
	cgram := flag.Bool("cgram", false, "display custom chars in cgram")
	syms := flag.Bool("syms", false, "display some built-in symbols")

	cpuprofile := flag.String("cpuprofile", "", "write cpu profile to `file` (for go tool pprof)")
	memprofile := flag.String("memprofile", "", "write memory profile to `file` (for go tool pprof)")
	traceout := flag.String("trace", "", "trace to `file` (for go tool trace)")
	flag.Parse()

	if *traceout != "" {
		//go tool trace
		f, err := os.Create(*traceout)
		if err != nil {
			panic(err)
		}
		defer f.Close()

		err = trace.Start(f)
		if err != nil {
			panic(err)
		}
		defer trace.Stop()
	}
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			panic(err)
		}
		err = pprof.StartCPUProfile(f)
		if err != nil {
			fmt.Println("start profile:", err)
		}
		defer pprof.StopCPUProfile()
	}
	if *memprofile != "" {
		defer func() {
			f, err := os.Create(*memprofile)
			if err != nil {
				panic(err)
			}
			err = pprof.WriteHeapProfile(f)
			if err != nil {
				fmt.Println("heap profile:", err)
			}
			f.Close()
		}()
	}
	if *out != "" {
		f, err := os.Create(*out)
		if err != nil {
			panic(err)
		}
		defer f.Close()
		serial.Output = f
	}
	lcd, err := cfa.Find()
	if err != nil {
		panic(err)
	}
	defer lcd.Close()
	if *dbg != "0000" {
		flags := cfa.FlagsFromString(*dbg)
		lcd.Debug(flags)
	}
	fmt.Println("running...")
	if err := lcd.Clear(); err != nil {
		fmt.Println(err)
	}
	if err := lcd.DefaultBacklight(); err != nil {
		fmt.Println(err)
	}
	if *ping {
		pingLoop(lcd)
	}
	if *cursor {
		cycleCursor(lcd)
	}
	if *events {
		showEvents(lcd)
	}
	if *menu > 0 {
		showMenu(*menu, menuitems, lcd)
	}
	if *basics {
		basicsTest(lcd)
	}
	if *fade {
		if err := lcd.BlinkMsg(*msg, cfa.Fade, time.Second*2, time.Second*15); err != nil {
			fmt.Println(err)
		}
		if _, err := lcd.Msg("Done"); err != nil {
			fmt.Println(err)
		}
		time.Sleep(time.Second)
	}
	if *backlight {
		var i uint8
		for i = 0; i <= 100; i += 5 {
			if err := lcd.SetBacklight(i); err != nil {
				fmt.Println(err)
			}
			if _, err := lcd.Msg(fmt.Sprintf("bl %d", i)); err != nil {
				fmt.Println(err)
			}
			time.Sleep(time.Second)
		}
		if _, err := lcd.Msg("Done"); err != nil {
			fmt.Println(err)
		}
		if err := lcd.DefaultBacklight(); err != nil {
			fmt.Println(err)
		}
		time.Sleep(time.Second)
	}
	if *spin {
		spinner(lcd)
		if _, err := lcd.Msg("Done"); err != nil {
			fmt.Println(err)
		}
		time.Sleep(time.Second)
	}
	if *multi {
		testStuff(lcd, *msg)
	}
	if *pressAny {
		pressAKey(lcd)
		time.Sleep(time.Second)
		if _, err := lcd.Msg("Done"); err != nil {
			fmt.Println(err)
		}
	}
	if *long {
		if err := lcd.LongMsg(*msg, time.Second, time.Second*20); err != nil {
			fmt.Println(err)
		}
		time.Sleep(time.Second)
		if _, err := lcd.Msg("Done"); err != nil {
			fmt.Println(err)
		}
	}
	if *boot {
		bootMenu(lcd)
	}
	if *question {
		q, err := lcd.NewQuestion(cfa.LcdTxt("Are you really certain?"), cfa.Strs2LTxt("yes", "no"))
		if err != nil {
			panic(err)
		}
		ans := q.Ask(time.Second * 30)
		if _, err := lcd.Msg("Done"); err != nil {
			fmt.Println(err)
		}
		fmt.Printf("answer: %v\n", ans)
		time.Sleep(time.Second)
	}
	if *cgram {
		lines := [2]cfa.LcdTxt{}
		//there are only 8 chars, though a quick glance at the cgrom table makes it look like 16 are available.
		for i := 0; i < 8; i++ {
			c := fmt.Sprintf("%x", i)
			lines[0] = append(lines[0], byte(c[0]))
			lines[1] = append(lines[1], byte(i))
		}
		if err := lcd.Write(cfa.Coord{Row: 0}, cfa.LcdTxt("CGRAM custom chars")); err != nil {
			fmt.Println(err)
		}
		if err := lcd.Write(cfa.Coord{Row: 1}, lines[0]); err != nil {
			fmt.Println(err)
		}
		if err := lcd.Write(cfa.Coord{Row: 2}, lines[1]); err != nil {
			fmt.Println(err)
		}
	}
	if *syms {
		if err := lcd.Clear(); err != nil {
			fmt.Println(err)
		}
		line := cfa.LcdTxt{
			cfa.SymRight,
			cfa.SymLeft,
			cfa.SymDoubleUp,
			cfa.SymDoubleDown,
			cfa.SymLtLt,
			cfa.SymGtGt,
			cfa.SymUp,
			cfa.SymDown,
			cfa.SymCaret,
			cfa.SymCaron,
			cfa.SymFilled,
			cfa.SymSpace,
		}
		var nums cfa.LcdTxt
		for _, c := range line {
			n := fmt.Sprintf("%x", c%0x10)
			nums = append(nums, byte(n[0]))
		}
		if err := lcd.Write(cfa.Coord{}, nums); err != nil {
			fmt.Println(err)
		}
		if err := lcd.Write(cfa.Coord{Row: 1}, line); err != nil {
			fmt.Println(err)
		}
	}
}

func basicsTest(lcd *cfa.Lcd) {
	coords := lcd.MaxCursorPos()
	fmt.Printf("LCD width: %d, height: %d\n", coords.Col+1, coords.Row+1)
	if err := lcd.Write(cfa.Coord{0, 0}, cfa.LcdTxt("test message")); err != nil {
		fmt.Println(err)
	}
	time.Sleep(time.Second)
	if err := lcd.SetBacklight(10); err != nil {
		fmt.Println(err)
	}
	if err := lcd.Write(cfa.Coord{4, 1}, cfa.LcdTxt("message 2")); err != nil {
		fmt.Println(err)
	}
	time.Sleep(time.Second)
	if err := lcd.Write(cfa.Coord{0, 0}, cfa.LcdTxt("Doing important     ")); err != nil {
		fmt.Println(err)
	}
	time.Sleep(time.Second)
	//get lcd model
	fmt.Println(lcd.Revision())
	if err := lcd.Write(cfa.Coord{0, 0}, cfa.LcdTxt("Press keys (5s inactivity -> exit)")); err != nil {
		fmt.Println(err)
	}
	lastActivity := time.Now()
	for {
		key := lcd.Event()
		if key != cfa.KEY_NO_KEY {
			lastActivity = time.Now()
			fmt.Printf("got key %x\n", key)
		} else if lastActivity.Add(5 * time.Second).Before(time.Now()) {
			if err := lcd.Write(cfa.Coord{0, 0}, cfa.LcdTxt("Timeout - bye!")); err != nil {
				fmt.Println(err)
			}
			break
		}
		time.Sleep(time.Second)
	}
}

var menuitems = []cfa.LcdTxt{
	cfa.LcdTxt("Resume normal boot"),
	cfa.LcdTxt("Power off"),
	cfa.LcdTxt(""),
	cfa.LcdTxt("Factory Restore to latest image"),
	cfa.LcdTxt("Factory Restore to original image"),
	cfa.LcdTxt("Factory Restore (choose image)"),
	cfa.LcdTxt(""),
	cfa.LcdTxt("Data Erase"),
}

func showMenu(n int, items []cfa.LcdTxt, lcd *cfa.Lcd) {
	fmt.Println("menu with", n, "elements")
	for n > len(items) {
		e := cfa.LcdTxt(fmt.Sprintf("entry %d", len(items)))
		items = append(items, e)
	}
	delay := time.Second * 10
	if n > 2 {
		delay += time.Duration(n * 10)
	}
	selection := lcd.Menu(items[:n], delay, false)
	fmt.Println("result", selection)
	sel := ""
	switch selection {
	case cfa.CHOICE_CANCEL:
		sel = "(CANCEL)"
	case cfa.CHOICE_NONE:
		sel = "(NONE)"
	default:
		sel = `"` + string(items[selection]) + `"`
	}
	fmt.Println("item", sel)
}

func showEvents(lcd *cfa.Lcd) {
	for {
		key := lcd.WaitForEvent(time.Second)
		if key == cfa.KEY_NO_KEY {
			fmt.Println("no events")
			continue
		}
		fmt.Printf("got key %02x\n", key)
	}
}

func cycleCursor(lcd *cfa.Lcd) {
	lcd.SetCursorPosition(cfa.Coord{Row: 1, Col: 1})
	var s byte
	for {
		if err := lcd.Write(cfa.Coord{Col: 0, Row: 1}, cfa.LcdTxt(fmt.Sprintf("style %d", s))); err != nil {
			fmt.Println(err)
		}
		lcd.SetCursorStyle(s)
		s += 1
		if s > 4 {
			s = 0
		}
		time.Sleep(time.Second * 3)
	}
}

func pingLoop(lcd *cfa.Lcd) {
	for {
		s, e := lcd.Ping()
		if e != nil {
			fmt.Printf("error %s\n", e)
		} else if !s {
			fmt.Println("failed")
		}
	}
}

func spinner(lcd *cfa.Lcd) {
	sp := cfa.Spinner{
		Msg: "Doing important things...",
		Lcd: lcd,
	}
	err := sp.Display()
	if err != nil {
		fmt.Println(err)
	}
	for i := 0; i < 10; i++ {
		sp.Next()
		time.Sleep(time.Second)
	}
}

func testStuff(lcd *cfa.Lcd, msg string) {
	if _, err := lcd.Msg("LCD test..."); err != nil {
		fmt.Println(err)
	}
	time.Sleep(time.Second * 5)
	fmt.Println("flash message")
	if err := lcd.BlinkMsg(msg, cfa.Flash, time.Second*2, time.Second*20); err != nil {
		fmt.Println(err)
	}
	time.Sleep(time.Second * 5)
	fmt.Println("fade message")
	if err := lcd.BlinkMsg(msg, cfa.Fade, time.Second*2, time.Second*20); err != nil {
		fmt.Println(err)
	}
	time.Sleep(time.Second * 5)
	fmt.Println("plain message")
	if err := lcd.LongMsg(msg, time.Second*2, time.Second*20); err != nil {
		fmt.Println(err)
	}
	time.Sleep(time.Second * 5)
	fmt.Println("spinner")
	spinner(lcd)
	if _, err := lcd.Msg("LCD test done"); err != nil {
		fmt.Println(err)
	}
	fmt.Println("done")
}

func pressAKey(lcd *cfa.Lcd) {
	pressed, err := lcd.PressAnyKey("normal boot", 2*time.Second, 10*time.Second)
	if err != nil {
		fmt.Println("error:", err)
	}
	if pressed {
		fmt.Println("interrupted")
	} else {
		fmt.Println("not interrupted")
	}
}

var images = []cfa.LcdTxt{
	cfa.LcdTxt("PRODUCT.Os.Plat.2015-02-03.1593"),
	cfa.LcdTxt("PRODUCT.Os.Plat.2019-01-29.7900"),
	cfa.LcdTxt("PRODUCT.Os.Plat.2019-02-07.7936"),
	cfa.LcdTxt("PRODUCT.Os.Plat.2019-02-20.7978"),
	cfa.LcdTxt("PRODUCT.Os.Plat.2019-02-20.7980"),
	cfa.LcdTxt("PRODUCT.Os.Plat.2019-03-18.8060"),
}

func bootMenu(lcd *cfa.Lcd) {
	defer fmt.Println("Done")
	defer time.Sleep(time.Second)
	pressed, err := lcd.PressAnyKey("normal boot", 2*time.Second, 10*time.Second)
	if err != nil {
		fmt.Println("error:", err)
	}
	if !pressed {
		if _, err := lcd.Msg("Continuing normal boot..."); err != nil {
			fmt.Println(err)
		}
		fmt.Println("Normal boot")
		return
	}
	time.Sleep(time.Second)
	lcd.FlushEvents()
	choice, _ := lcd.MenuWithConfirm("boot option", menuitems, time.Minute*5, time.Minute, false)
	//confirmation answer is the 2nd ret val, but we really don't need it - just the choice
	switch choice {
	case cfa.CHOICE_CANCEL:
		if _, err := lcd.Msg("Canceled, continuing normal boot..."); err != nil {
			fmt.Println(err)
		}
		fmt.Println("Normal boot")
		return
	case cfa.CHOICE_NONE:
		if _, err := lcd.Msg("Time out, continuing normal boot..."); err != nil {
			fmt.Println(err)
		}
		fmt.Println("Normal boot")
		return
	default:
		if len(menuitems[choice]) == 0 {
			if _, err := lcd.Msg("Invalid selection, continuing normal boot..."); err != nil {
				fmt.Println(err)
			}
			fmt.Println("Normal boot")
			return
		}
	}
	if err := lcd.LongMsg("Selected: "+string(menuitems[choice]), time.Second/2, 2*time.Second); err != nil {
		fmt.Println(err)
	}
	time.Sleep(time.Second)
	if choice == 5 {
		//choose image
		if err := lcd.LongMsg("Starting factory restore, please wait...", time.Second/2, 2*time.Second); err != nil {
			fmt.Println(err)
		}
		img, _ := lcd.MenuWithConfirm("image to restore", images, 5*time.Minute, time.Minute, false)
		switch img {
		case cfa.CHOICE_CANCEL:
			if _, err := lcd.Msg("Canceled, reboot..."); err != nil {
				fmt.Println(err)
			}
			fmt.Println("Normal boot")
			return
		case cfa.CHOICE_NONE:
			if _, err := lcd.Msg("Time out, reboot..."); err != nil {
				fmt.Println(err)
			}
			fmt.Println("Normal boot")
			return
		default:
			fmt.Println("Factory restore with image", string(images[img]))
			if err := lcd.LongMsg("Img "+string(images[img]), time.Second/2, 2*time.Second); err != nil {
				fmt.Println(err)
			}
		}
	}
}
