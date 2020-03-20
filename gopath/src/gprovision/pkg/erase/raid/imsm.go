// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package raid

import (
	"bytes"
	"io"
	//"recovery/log"
)

const (
	imsmRecordSize = 1024
)

func (d *Device) detectIMSM(buf []byte) bool {
	//log.Logf("enter detectIMSM() for %s", d.dev)
	imsmSig := []byte("Intel Raid ISM Cfg Sig. ")
	if buf == nil || len(buf) < imsmRecordSize {
		panic("nil or short buffer")
	}
	buf = buf[len(buf)-imsmRecordSize:]
	return bytes.Equal(buf[:len(imsmSig)], imsmSig)
}

func (d *Device) saveIMSM() (err error) {
	d.arrayMetadata, err = d.AlignedRead(-1*imsmRecordSize, imsmRecordSize)
	if len(d.arrayMetadata) != imsmRecordSize && err == nil {
		err = io.ErrShortWrite
	}
	return
}

func (d *Device) restoreIMSM() (err error) {
	off := int64(-1 * len(d.arrayMetadata))
	err = d.AlignedWrite(off, d.arrayMetadata)

	return
}
