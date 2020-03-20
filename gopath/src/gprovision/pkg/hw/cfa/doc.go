// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

/*
Package cfa implements communication with Crystalfontz LCDs such as
CFA-631 and CFA-635, without use of CGO. In addition, this package supports
keypress events and menus.


Packet Structure, from data sheet

The following can be found beginning on Pg 34, CFA631_Data_Sheet_Release_2014-11-17.pdf.

All packets have the following structure:
 <type><data_length><data><CRC>

type is one byte, and identifies the type and function of the packet:
 TTcc cccc
 |||| ||||--Command, response, error or report code 0-63
 ||---------Type:
            00 = normal command from host to CFA631
            01 = normal response from CFA631 to host
            10 = normal report from CFA631 to host (not in direct response to a command from the host)
            11 = error response from CFA631 to host (a packet with valid structure but illegal content was received by the CFA631)

data_length specifies the number of bytes that will follow in the data field. The valid range of data_length is 0 to 22.

data is the payload of the packet. Each type of packet will have a specified data_length and format for data as well as algorithms for decoding data detailed below.

CRC is a standard 16-bit CRC of all the bytes in the packet except the CRC itself. The CRC is sent LSB first. At the port, the CRC immediately follows the last used element of data []. See Sample Algorithms To Calculate The CRC (Pg. 66) for details.

The following C definition may be useful for understanding the packet structure.
 typedef struct
 {
    unsigned char command;
    unsigned char data_length;
    unsigned char data[MAX_DATA_LENGTH];
    unsigned short CRC;
 } COMMAND_PACKET;

Packet Notes

While the documentation can be interpreted as saying the packet size is fixed,
it is not; there is never padding between the last valid data byte and the crc,
and the packet length is always data_length+4.

Crystalfontz claims above that the CRC used is standard, but a bit of googling
leads me to the conclusion that there is no such thing. There are myriad
variations of 16-bit CRC with different constants, and the sites discussing it
tend to disagree on which constants are used by which protocols. Very few
standards actually include any test vectors. Crystalfontz' own data sheets for
the 631 and 635 include a test vector in one example... but the output listed
does not match the value computed by the "crystalfontz linux example", which is
able to talk to the LCD.

Known Issues

The CFA-631 and XES-635BK-TML-KU work fine, but the XES-635BK-TMF-KU can
hang or otherwise lose packets. By far the most effective workaround seems to
be to minimize the number of packets sent. Of course, that's not possible beyond
a certain point without throwing usability out the window.

We have shipped some XES-635BK-TFE-KU. The only difference from the TMF should be
the display/backlight color, so those are likely to have the same issues as TMF.

Buttons and events

Key event reports (and responses to the key poll command) are written to an
event channel for async read.

Only key releases are considered. Reading key press events or keys being held
down (only seen when polling) would not be difficult, but handling them with the
menu would complicate things.
*/
package cfa
