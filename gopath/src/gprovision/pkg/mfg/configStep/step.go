// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

// Package configStep implements manufacturing steps that can be specified in manufData.json.
// This step can have one or more downloadable files and one or more executed commands.
// It can be run at different points in the manufacturing process, and individual command
// success/failure can be required or ignored.
//
// Files with suffix '.tar.xz' or '.txz' are automatically extracted into the temp dir.
//
// Commands first have templating resolved, then are split into args via github.com/google/shlex.
// Commands for a step are executed in order. Steps with the same When value are executed in the
// order listed in json.
package configStep

import (
	"bytes"
	"fmt"
	"gprovision/pkg/log"
	"gprovision/pkg/net/xfer"
	"io/ioutil"
	"os"
	"os/exec"
	fp "path/filepath"
	"strings"
	"text/template"
	"unicode"

	"github.com/google/shlex"
)

type WhenType int

const (
	RunBeforeQA WhenType = iota
	RunAfterQA
	RunBeforeImaging
	RunAfterImaging
	RunBeforeMfg
	RunAfterMfg
	RunBeforePWSet
	RunAfterPWSet
)

func (wt *WhenType) UnmarshalJSON(b []byte) error {
	switch strings.ToLower(strings.Trim(string(b), `"`)) {
	case "beforeqa":
		fallthrough
	case "runbeforeqa":
		*wt = RunBeforeQA
	case "afterqa":
		fallthrough
	case "runafterqa":
		*wt = RunAfterQA
	case "beforeimaging":
		fallthrough
	case "runbeforeimaging":
		*wt = RunBeforeImaging
	case "afterimaging":
		fallthrough
	case "runafterimaging":
		*wt = RunAfterImaging
	case "beforemfg":
		fallthrough
	case "runbeforemfg":
		*wt = RunBeforeMfg
	case "aftermfg":
		fallthrough
	case "runaftermfg":
		*wt = RunAfterMfg
	case "beforepwset":
		fallthrough
	case "runbeforepwset":
		*wt = RunBeforePWSet
	case "afterpwset":
		fallthrough
	case "runafterpwset":
		*wt = RunAfterPWSet
	default:
		return fmt.Errorf("unable to translate %s into a WhenType", string(b))
	}
	return nil
}

type ConfigSteps []Step

func (c ConfigSteps) RunApplicable(When WhenType) (success bool) {
	var err error
	for _, s := range c {
		if s.When == When {
			err = s.Run()
			if err != nil {
				log.Logf("Error executing Step %s: %s", s.Name, err)
				return false
			}
		}
	}
	return true
}

// A set of config steps for a platform.
type PlatformConfig struct {
	DevCodeName string
	ConfigSteps ConfigSteps
}
type PlatformConfigs []PlatformConfig

func (configs PlatformConfigs) Find(codeName string) (cs ConfigSteps) {
	for _, cfg := range configs {
		if cfg.DevCodeName == codeName {
			cs = cfg.ConfigSteps
			break
		}
	}
	return
}

type ExitStatus int

const (
	ESMustSucceed ExitStatus = iota
	ESDontCare
	ESMustFail
)

func (es *ExitStatus) UnmarshalJSON(b []byte) error {
	switch strings.ToLower(strings.Trim(string(b), `"`)) {
	case "mustsucceed":
		fallthrough
	case "esmustsucceed":
		*es = ESMustSucceed
	case "dontcare":
		fallthrough
	case "esdontcare":
		*es = ESDontCare
	case "mustfail":
		fallthrough
	case "esmustfail":
		*es = ESMustFail
	default:
		return fmt.Errorf("unable to translate %s into an exit status type", string(b))
	}
	return nil
}

/* A command to be executed during a Step. Command, AddPath, and AddLibPath are subject
** to template expansion using the values in StepData (which includes CommonData).
** Templating is via golang's package text/template.
**
** Example: Print the serial number
** Command could be "echo 'Serial number is {{.Serial}}'".
** If the serial is SN123, the command would print "Serial number is SN123".
**
** Example: Execute a binary auto-extracted from a .tar.xz file
** Command could be "{{.DLDir}}/path/in/tar/to/binary --arg --arg2"
** {{.DLDir}} would be replaced with the temp dir, resulting in an absolute path, and
** the executable is executed (assuming correct file mode).
**
** Note that passwords won't be available until close to the password-setting step.
** Also note that AddPath only applies to additional executables used by Command, and cannot be used to search for Command itself.
** AddLibPath, however, _will_ apply to Command.
 */
type StepCmd struct {
	ExitStatus          ExitStatus
	Command             string
	AddPath, AddLibPath string
}

//Data usable in step templates, but not unique to any one step
type CommonData struct {
	RecoveryDir string // where RECOVERY volume is mounted
	Serial      string // unit serial number
	BiosPass    string // what the bios password has been/will be set to
	IpmiPass    string // ditto, for ipmi
	OSPass      string // ditto, for OS
}

//All data for step templates, including step-specific data (currently only DLDir)
type StepData struct {
	*CommonData
	DLDir string //temp dir where file(s) for this step were downloaded
}

var CommonTemplateData CommonData

//call once passwords are known
func AddPWs(biospass, ipmipass, ospass string) {
	CommonTemplateData.BiosPass = biospass
	CommonTemplateData.IpmiPass = ipmipass
	CommonTemplateData.OSPass = ospass
}

// A Step specifies a sequence of 0 or more file downloads followed by 0 or more command executions.
// Commands are subject to template expansion.
type Step struct {
	Name     string
	When     WhenType
	Files    []xfer.TVFile
	Commands []StepCmd
	Verbose  bool
	tmplData StepData
}

// Run takes actions necessary to complete a step. That is, it downloads listed files and then runs listed
// commands. Any command whose exit code does not match the specified value causes Run() to exit with error.
func (s *Step) Run() (err error) {
	s.tmplData.CommonData = &CommonTemplateData
	if len(s.Files) > 0 {
		safeName := makeFsSafeName(s.Name)
		s.tmplData.DLDir, err = ioutil.TempDir("", safeName)
		if err != nil {
			return fmt.Errorf("failed to create temp dir for configStep %s: %s", s.Name, err)
		}
		defer os.RemoveAll(s.tmplData.DLDir)
		for _, f := range s.Files {
			f.Dest = fp.Join(s.tmplData.DLDir, f.Basename())
			err = f.GetWithRetry()
			if err != nil {
				return fmt.Errorf("Step %s: failed to download %s: %s", s.Name, f.Basename(), err)
			}
			if strings.HasSuffix(f.Dest, ".txz") || strings.HasSuffix(f.Dest, ".tar.xz") {
				log.Logf("Step %s: extracting %s in place", s.Name, f.Basename())
				extract := exec.Command("tar", "xJf", f.Dest, "-C", s.tmplData.DLDir)
				_, success := log.Cmd(extract)
				if !success {
					return fmt.Errorf("failed to extract %s for configStep %s", f.Basename(), s.Name)
				}
			}
		}
	}
	for _, c := range s.Commands {
		err = s.runCmd(c)
		if err != nil {
			break
		}
	}
	return
}

//Returns a copy of a string, with non-fs-safe and non-lexer-safe characters replaced.
// Returned value will be at most 20 characters.
func makeFsSafeName(in string) (out string) {
	if len(in) > 20 {
		in = in[:20]
	}
	return strings.Map(func(r rune) rune {
		if r < '#' || r > 'z' || unicode.IsSpace(r) {
			return '_'
		}
		switch r {
		case '*':
			fallthrough
		case '?':
			fallthrough
		case ':':
			fallthrough
		case '\\':
			fallthrough
		case '/':
			return '_'
		default:
			return r
		}
	}, in)
}

var (
	EEXECSUCCESS = fmt.Errorf("Execution succeeded but must fail")
	EEXECFAIL    = fmt.Errorf("Execution failed but must succeed")
)

func (s *Step) runCmd(c StepCmd) error {
	out, err := s.applyTmpl(c.Command)
	if err != nil {
		return err
	}
	args, err := shlex.Split(out)
	if err != nil {
		return err
	}
	cmd := exec.Command(args[0])
	cmd.Args = args
	if c.AddPath != "" {
		p, err := s.applyTmpl(c.AddPath)
		if err != nil {
			return err
		}
		addEnv(cmd, "PATH", p, true)
	}
	if c.AddLibPath != "" {
		l, err := s.applyTmpl(c.AddLibPath)
		if err != nil {
			return err
		}
		addEnv(cmd, "LD_LIBRARY_PATH", l, true)
	}
	out, success := log.Cmd(cmd)
	if success && s.Verbose {
		log.Logf("command output: %s", out)
	}
	if success && c.ExitStatus == ESMustFail {
		err = EEXECSUCCESS
	} else if !success && c.ExitStatus == ESMustSucceed {
		err = EEXECFAIL
	}
	return err
}

func (s *Step) applyTmpl(in string) (out string, err error) {
	var tmpl *template.Template
	tmpl, err = template.New("").Parse(in)
	if err != nil {
		if s.Verbose {
			log.Logf("Step %s: Error parsing templated command %s: %s", s.Name, in, err)
		}
		return
	}
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, s.tmplData)
	if err != nil {
		if s.Verbose {
			log.Logf("Step %s: Error executing templated command %s: %s", s.Name, in, err)
		}
		return
	}
	out = buf.String()
	if s.Verbose {
		log.Logf("Template expansion in %s: %s -> %s", s.Name, in, out)
	}
	return
}

// Add/overwrite/prepend env var. If var doesn't exist it is created, else it is
// overwritten if prepend is false. If prepend is true, content of val is inserted
// at the beginning, followed by a colon and the existing content.
func addEnv(cmd *exec.Cmd, vname, val string, prepend bool) {
	if cmd.Env == nil {
		cmd.Env = []string{vname + "=" + val}
		return
	}
	for i, e := range cmd.Env {
		sp := strings.SplitN(e, "=", 2)
		if len(sp) != 2 {
			continue
		}
		if sp[0] == vname {
			val += ":" + sp[1]
			cmd.Env[i] = vname + "=" + val
			return
		}
	}
	cmd.Env = append(cmd.Env, vname+"="+val)
}
