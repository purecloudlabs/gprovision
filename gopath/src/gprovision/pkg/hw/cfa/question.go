// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package cfa

import (
	"fmt"
	"gprovision/pkg/log"
	"time"
)

const (
	maxTimeouts = 2
)

//Like menu, but confirmation required. Re-displays menu if choice is not confirmed.
func (l *Lcd) MenuWithConfirm(desc string, items []LcdTxt, menuTime, confirmTime time.Duration, keyPolling bool) (Choice, Answer) {
	if l == nil {
		time.Sleep(menuTime + confirmTime)
		return CHOICE_NONE, ANSWER_NA
	}
	var confirm Answer
	var choice Choice
	timeoutCount := 0
	for confirm != ANSWER_YES {
		choice = l.Menu(items, menuTime, keyPolling)
		confirm = l.ConfirmChoice(desc, items, choice, confirmTime)
		if choice == CHOICE_NONE && confirm == ANSWER_NA {
			timeoutCount++
			log.Logf("MenuWithConfirm(%s,...): timeout", desc)
		}
		if timeoutCount >= maxTimeouts {
			log.Logf("MenuWithConfirm(%s,...): max timeouts", desc)
			break
		}
	}
	return choice, confirm
}

type Answer int

const (
	ANSWER_NA Answer = iota //no choice made - timeout
	ANSWER_NO
	ANSWER_YES
)

//From %s, you chose %s. Are you certain?
//   >No< Yes
func (l *Lcd) ConfirmChoice(desc string, items []LcdTxt, choice Choice, timeout time.Duration) Answer {
	if l == nil {
		time.Sleep(timeout)
		return ANSWER_NA
	}
	if int(choice) >= len(items) || choice < -2 {
		//out of range
		return ANSWER_NA
	}
	var item LcdTxt
	switch choice {
	case CHOICE_NONE:
		item = LcdTxt("(timeout - go back)")
	case CHOICE_CANCEL:
		item = LcdTxt("to cancel (go back)")
	default:
		item = items[choice]
	}
	txt := LcdTxt(fmt.Sprintf("From %s, you chose %s. Are you certain?", desc, item))
	return l.YesNo(txt, timeout)
}

func (l *Lcd) YesNo(txt LcdTxt, timeout time.Duration) Answer {
	if l == nil {
		time.Sleep(timeout)
		return ANSWER_NA
	}
	q, err := l.NewQuestion(txt, []LcdTxt{LcdTxt("No"), LcdTxt("Yes")})
	if err != nil {
		log.Logf("question error: %s", err)
		return ANSWER_NA
	}
	choice := q.Ask(timeout)
	var ans Answer
	switch choice {
	case 0:
		ans = ANSWER_NO
	case 1:
		ans = ANSWER_YES
	default:
		ans = ANSWER_NA
	}
	return ans
}

type Question struct {
	l        *Lcd
	txt      LcdTxt
	buttons  radioButtonSet
	choice   Choice
	debug    bool
	syncTick chan<- time.Time
}

func (l *Lcd) NewQuestion(txt LcdTxt, opts []LcdTxt) (*Question, error) {
	var err error
	q := &Question{
		l:   l,
		txt: txt,
	}
	//set up buttons, which must fit on one line
	err = q.createButtonSet(opts)
	if err != nil {
		return nil, err
	}
	return q, nil
}

const askUpdateCycle = time.Second

func (q *Question) Ask(timeout time.Duration) Choice {
	done := make(chan struct{})
	update := NewTicker(askUpdateCycle)
	go func() {
		time.Sleep(timeout)
		close(done)
	}()
	return q.ask(done, update)
}
func (q *Question) ask(done chan struct{}, update *Ticker) Choice {
	if q.l == nil {
		<-done
		return CHOICE_NONE
	}
	q.l.mutex.Lock()
	defer q.l.mutex.Unlock()
	err := q.l.clear()
	if err != nil && q.l.DbgGeneral {
		log.Logf("Question error: %s", err)
	}
	q.l.setLegend(Legend_VRX, true)
	defer q.l.setLegend(LegendNone, true)
	q.l.setupKeyReporting(false, true)
	area := q.l.dims
	b := NewBlurb(q.l, q.txt, Coord{}, Coord{Row: area.Row - 1, Col: q.l.Width()})
	err = q.l.write(Coord{Row: area.Row}, q.buttons.render(), true)
	if err != nil && q.l.DbgGeneral {
		log.Logf("Question error: %s", err)
	}
	//loop until timeout, allowing q.txt to scroll horiz if necessary
	q.choice = CHOICE_NONE
	for q.choice == CHOICE_NONE {
		select {
		case <-done:
			return CHOICE_NONE
		default:
		}
		_, err = b.draw(false)
		if err != nil && q.l.DbgGeneral {
			log.Logf("Question error: %s", err)
		}
		if q.buttons.prev != q.buttons.selection {
			q.buttons.prev = q.buttons.selection
			err = q.l.write(Coord{Row: area.Row}, q.buttons.render(), true)
			if err != nil && q.l.DbgGeneral {
				log.Logf("Question error: %s", err)
			}
		}
		evt := q.l.waitForEvent(update.C)
		if evt != KEY_NO_KEY {
			q.handleEvent(evt)
		}
		if q.syncTick != nil {
			q.syncTick <- time.Now()
		}
	}
	return q.choice
}

//Handle up/down/enter/exit
func (q *Question) handleEvent(k KeyActivity) {
	max := len(q.buttons.btns) - 1
	switch k {
	case KEY_LL_RELEASE:
		fallthrough
	case KEY_RIGHT_RELEASE:
		if q.buttons.selection >= max {
			//wrap around
			q.buttons.selection = 0
			q.l.setLegend(Legend_VRX, false)
		} else {
			q.buttons.selection++
			if q.buttons.selection == max {
				q.l.setLegend(LegendLV_X, false)
			} else {
				q.l.setLegend(LegendLVRX, false)
			}
		}
	case KEY_UL_RELEASE:
		fallthrough
	case KEY_LEFT_RELEASE:
		if q.buttons.selection <= 0 {
			q.buttons.selection = max
			q.l.setLegend(LegendLV_X, false)
		} else {
			q.buttons.selection--
			if q.buttons.selection == 0 {
				q.l.setLegend(Legend_VRX, false)
			} else {
				q.l.setLegend(LegendLVRX, false)
			}
		}
	case KEY_UR_RELEASE:
		fallthrough
	case KEY_ENTER_RELEASE:
		q.choice = Choice(q.buttons.selection)
	case KEY_LR_RELEASE:
		fallthrough
	case KEY_EXIT_RELEASE:
		q.choice = CHOICE_CANCEL
	default:
		if q.debug {
			log.Logf("Question.handleEvent: ignoring key code 0x%02x", k)
		}
	}
}

//an item to be displayed, only one of which can be selected at a time
type radioButton struct {
	txt LcdTxt
}

func (rb *radioButton) render(selected bool, st *styleSet) LcdTxt {
	style := st.deselected
	if selected {
		style = st.selected
	}
	l := LcdTxt{style[0]}
	l = append(l, rb.txt...)
	l = append(l, style[1])
	return l
}

//set of radio buttons. must fit on one row of display.
type radioButtonSet struct {
	btns      []*radioButton
	styles    *styleSet
	selection int
	prev      int //used to track when buttons need redrawn
}

func (q *Question) createButtonSet(opts []LcdTxt) error {
	//determine if buttons can all fit on a line
	//if not, error
	w := q.l.Width()
	if w < rbsMinWidth(opts) {
		return ERange
	}
	for _, opt := range opts {
		q.buttons.btns = append(q.buttons.btns, &radioButton{txt: opt})
	}
	q.buttons.styles = &styleSet{selected: wrapArrows, deselected: wrapNone}
	q.buttons.prev = -1
	return nil
}

//calculates width, with one space on each end and two between items
func rbsMinWidth(opts []LcdTxt) byte {
	var w byte
	for _, opt := range opts {
		w += byte(len(opt)) + 2
	}
	return w
}

func (rbs *radioButtonSet) render() LcdTxt {
	var line LcdTxt
	for i, rb := range rbs.btns {
		selection := false
		if rbs.selection == i {
			selection = true
		}
		line = append(line, rb.render(selection, rbs.styles)...)
	}
	return line
}

type wrapStyle [2]byte

var (
	wrapNone     = wrapStyle{0x20, 0x20} //spaces
	wrapBrackets = wrapStyle{0xfa, 0xfc} // [ and ] - not ascii
	wrapArrows   = wrapStyle{0x10, 0x11} // like > and < , but filled in
	wrapCorners  = wrapStyle{0x96, 0x97} // corner brackets (apparently used as quotes in Japanese, Chinese, Korean)
)

type styleSet struct {
	selected, deselected wrapStyle
}
