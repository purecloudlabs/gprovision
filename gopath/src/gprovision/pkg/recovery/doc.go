// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

// Package recovery, known alternately as factory restore, is used to overwrite
// the main drive of a unit with a fresh image. This can happen due to the cloud
// pushing a new os image, due to the user requesting a factory restore, or due
// to the main drive being erased or corrupted.
//
// Recovery
//
// (AKA Factory Restore)
//
// 	* identifies the hardware it's running on
// 	* determines which device(s) are to be used for the main volume and which
//    is recovery
// 	* configures system as necessary
// 	    * change bios options such as fake raid (if the platform requires it and
//        tools are present)
// 	    * set up linux raid (if the platform requires it)
// 	    * partitioning (legacy vs uefi)
// 	* write image, unit info, boot files
// 	* reboot
//
// Images
//
// Images will be located within the /Image/ directory on the recovery volume
// and have the `.upd` extension. Images are xz-compressed tar files, and must
// have the XZ signature and have been compressed with the option to use the
// SHA256 checksum. This signature and checksum type are verified during image
// validation.
//
// Restore process
//
// step by step
//
//	 * Factory restore identifies the model of device it's running on
//	 * using that information, locates the recovery drive
//	 * load factory restore config json, if it exists
//	 * look for update files in Image/ on recovery drive
//	 * sort updates, newest first
//	 * go through update list, checking integrity with xz's embedded SHA256 checksum
//	     * stop when first valid update is found
//	 * if a valid update has been found:
//	     * reconfigure BIOS (supported platforms), disabling fake raid
//	         * only happens during windows -> linux conversion
//	     * update is applied
//	 * if no valid update, reboot
//	 * read OS password from encrypted file, insert into etc/shadow
//	   * if file doesn't exist, serial number is hashed and used as password
//	 * delete factory restore config json, if it exists
//	 * create flag file on main drive
//	 * reboot
//
// Magic files
//
// If emergency mode file(s) are found by init, the paths are passed to factory restore.
// These files could be an image or a command file. A person **must** remain present when factory restore is processing a magic file, as there isn't a mechanism to prevent a boot loop.
//
// If multiple volumes are present and contain emergency file(s), only one volume's files will be considered. The order volumes are searched is outside our control and may not be repeatable.
//
// If one volume contains multiple emergency files, factory restore will take the list of files and split into a list of images and a list of command files. Any image will take precedence over any command file; if there are multiple images, the first found is chosen and all others are ignored. If there are no images and multiple command files, the one that is deemed to most closely match the device is chosen and others ignored.
//
// - Emergency Image
//
// Any file > 1MB is assumed to be an image; the same signature requirements apply as with normal images.
//
// If the file looks like an image, recovery enters emergency imaging mode. Prefixes strs.EmergPfx() and `_` (if they exist) are removed from the name, with the remainder used as the disktag. In emergency imaging mode, this image is processed as normal, except that no other images are considered - regardless of whether this image is older, corrupt, etc.
//
// - Emergency command file
//
// No opensource implementation currently exists.
//
package recovery
