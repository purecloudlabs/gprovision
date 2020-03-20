// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package integ

//common stuff shared between mfg integ test and dev vm's
import (
	"go/build"
	"gprovision/pkg/common/strs"
	"gprovision/pkg/log"
	gtst "gprovision/testing"
	"gprovision/testing/util"
	"gprovision/testing/vm"
	"io"
	"os"
	fp "path/filepath"
	"time"

	"github.com/u-root/u-root/pkg/cpio"
	"github.com/u-root/u-root/pkg/golang"
	"github.com/u-root/u-root/pkg/qemu"
	"github.com/u-root/u-root/pkg/uroot"
	"github.com/u-root/u-root/pkg/uroot/builder"
	"github.com/u-root/u-root/pkg/uroot/initramfs"
	"github.com/u-root/u-root/pkg/vmtest"
)

//Defines the base vm used for 'devvm' and lifecycle tests.
func BaselineVM(uefi bool, m int, tb gtst.TB, tmpdir string) *qemu.Options {
	additionalDevs := []qemu.Device{
		vm.SmBios1(uefi),
		&vm.BlockDev{
			Attachment: vm.UsbXhciAttach,
			Size:       10 * GB,
			Count:      1,
			TmpDir:     tmpdir,
			Id:         "recovery",
		},
		&vm.BlockDev{
			Attachment: vm.IdeAttach,
			Size:       20 * GB,
			Count:      1,
			TmpDir:     tmpdir,
			Model:      "roothdd",
		},
	}
	return vmOpts(uefi, m, tb, tmpdir, additionalDevs...)
}
func vmOpts(uefi bool, m int, tb gtst.TB, tmpdir string, additionalDevs ...qemu.Device) *qemu.Options {
	qopts := &qemu.Options{
		Devices: []qemu.Device{
			vm.Ram(m),
			vm.NoReboot{},
			vm.OUINic{},
			&qemu.VirtioRandom{},
		},
		Timeout: 40 * time.Second,
	}
	qopts.Devices = append(qopts.Devices, additionalDevs...)
	if uefi {
		qopts.Devices = append(qopts.Devices, vm.UefiFwSetup(tb, tmpdir))
	}
	if Lcd {
		dev, err := GetLcd()
		if err != nil {
			tb.Fatal(err)
		}
		qopts.Devices = append(qopts.Devices, dev)
	}
	return qopts
}

func Initramfs(tmpdir, combinedCpio string, tags []string, dirs ...string) *irfs {
	if len(combinedCpio) == 0 {
		combinedCpio = fp.Join(os.Getenv("INFRA_ROOT"), "work/combined.cpio")
	}

	if len(tags) == 0 {
		tags = []string{"release"}
	}
	ctx := build.Default
	ctx.BuildTags = tags
	ctx.CgoEnabled = false
	ctx.GOPATH = os.Getenv("GOPATH")

	var flist []string
	for _, d := range dirs {
		if len(d) == 0 {
			//found out the hard way
			log.Fatalf("empty string passed to Initramfs() - would add entire repo + workdir (5GB+) to initramfs. in %#v", dirs)
		}
		files, err := util.FileList(d)
		if err != nil {
			log.Fatalf("%s - %s", d, err)
		}
		flist = append(flist, files...)
	}

	f, err := os.Open(combinedCpio)
	if err != nil {
		log.Fatalf("%s", err)
	}
	combined := cpio.Newc.Reader(f)
	return &irfs{
		Opts: uroot.Opts{
			SkipLDD:     true,
			BaseArchive: combined,
			TempDir:     tmpdir,
			Commands: []uroot.Commands{
				uroot.Commands{
					Builder: &builder.BinaryBuilder{},
					Packages: []string{
						"gprovision/cmd/init",
					},
				},
			},
			Env:        golang.Environ{ctx},
			ExtraFiles: flist,
		},
	}
}

type irfs struct {
	uroot.Opts
}

//from vmtest.QEMU()
func (i *irfs) Build() (path string, err error) {
	if len(i.TempDir) == 0 {
		log.Fatalf("temp dir unset")
	}
	if i.BaseArchive == nil {
		log.Logf("using default archive")
		i.BaseArchive = uroot.DefaultRamfs.Reader()
	}
	if len(i.InitCmd) == 0 {
		i.InitCmd = "init"
	}

	logger := &UrootLoggerAdapter{}

	// OutputFile
	var outputFile string
	if i.OutputFile == nil {
		outputFile = fp.Join(i.TempDir, "initramfs.cpio")
		w, err := initramfs.CPIO.OpenWriter(logger, outputFile, "", "")
		if err != nil {
			return "", err
		}
		i.OutputFile = w
	}

	// Finally, create an initramfs!
	if err := uroot.CreateInitramfs(logger, i.Opts); err != nil {
		return "", err
	}
	return outputFile, nil
}

//vm options for a mfg vm, using test kernel + freshly-built initramfs
func Mfgopts(t gtst.TB, tmpdir, mfgurl string, serOut io.WriteCloser) (*vmtest.Options, error) {
	if M < 512 {
		//used to work with 256m. not sure what happened...
		t.Logf("%dM may be too little memory", M)
	}
	base := BaselineVM(false, M, t, tmpdir)
	irfs := Initramfs(tmpdir, "", []string{"mfg", "release"}, "initramfs", "initramfs_mfg")
	opts := &vmtest.Options{
		QEMUOpts:   *base,
		BuildOpts:  irfs.Opts,
		DontSetEnv: true,
	}
	opts.QEMUOpts.Devices = append(opts.QEMUOpts.Devices, vm.ArbitraryKArgs{"mfgurl=" + mfgurl})
	opts.QEMUOpts.SerialOutput = serOut
	return opts, nil
}

//vm options for a fr vm, using test kernel + freshly-built initramfs
func FRopts(t gtst.TB, tmpdir string) *vmtest.Options {
	if M < 512 {
		//used to work with 256m. not sure what happened...
		t.Logf("%dM may be too little memory", M)
	}

	qopts := vmOpts(
		false,
		M,
		t,
		tmpdir,
		vm.SmBios1p9(false),
		&vm.BlockDev{
			Attachment: vm.IdeAttach,
			Size:       5 * GB,
			Count:      1,
			TmpDir:     tmpdir,
			Model:      "roothdd",
		},
	)
	irfs := Initramfs(tmpdir, "", []string{"release"}, "initramfs")
	opts := &vmtest.Options{
		QEMUOpts:   *qopts,
		BuildOpts:  irfs.Opts,
		DontSetEnv: true,
	}
	opts.QEMUOpts.SerialOutput = nil
	return opts
}

func EraseOpts(t gtst.TB, tmpdir string) *qemu.Options {
	args := vm.ArbitraryKArgs([]string{
		strs.EraseEnv() + "=1",
		strs.VerboseEnv() + "=1",
		//following are added by vmtest.QEMUTest(), which we are not using
		"console=ttyS0",
		"earlyprintk=ttyS0",
	})
	qopts := vmOpts(
		false,
		M,
		t,
		tmpdir,
		vm.SmBios1p9(false),
		&vm.BlockDev{
			Attachment: vm.IdeAttach,
			Size:       5 * GB,
			Count:      1,
			TmpDir:     tmpdir,
			Model:      "roothdd",
		},
		&args,
	)
	qopts.Timeout = 2 * time.Minute
	return qopts
}
