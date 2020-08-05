// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

/* Package history logs image success/failure to disk.
Data logged includes image name and success/failure counts for imaging and first boot.

IMPORTANT When using in recovery, must add history.RebootHook to log.Preboot.
*/
package history

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	fp "path/filepath"
	"strings"
	"time"

	"github.com/purecloudlabs/gprovision/pkg/common/strs"
	dt "github.com/purecloudlabs/gprovision/pkg/disktag"
	futil "github.com/purecloudlabs/gprovision/pkg/fileutil"
	"github.com/purecloudlabs/gprovision/pkg/log"
)

const (
	histName = "recovery_history.json"
)

var (
	histPath          string         //path to history file
	MaxFailuresPerImg uint       = 5 //max allowed sum of ImagingFailures and BootFailures
	results           ResultList     //list of images and whether they seem to be good
)

type ImageResult struct {
	Image           string   //image name
	ImagingAttempts uint     `json:",omitempty"` //currently only written, not used
	ImagingFailures uint     `json:",omitempty"`
	BootAttempts    uint     `json:",omitempty"` //currently only written, not used
	BootFailures    uint     `json:",omitempty"`
	Notes           []string `json:",omitempty"` //record timestamp+reason for failure
}
type ResultList []*ImageResult

//makes the json look nice
type serializationFmt struct {
	ImageResults ResultList
}

//Sets where the history file can be found. Path is stored for use by other functions in the package.
func SetRoot(path string) {
	dir := fp.Join(path, strs.RecoveryLogDir())
	err := os.MkdirAll(dir, 0777)
	if err != nil {
		log.Logf("error %s creating dir %s for %s", err, dir, histName)
	}
	histPath = fp.Join(dir, histName)
}

func Rollover(path string) {
	if len(histPath) == 0 {
		SetRoot(path)
	}
	old := histPath + ".prev"
	err := os.Remove(old)
	if err != nil && !os.IsNotExist(err) {
		log.Logf("history log - removing %s: %s", old, err)
	}
	err = os.Rename(histPath, old)
	if err != nil && !os.IsNotExist(err) {
		log.Logf("history log - roll %s: %s", histPath, err)
	}
}

//Reads history file, comparing stored count with MaxFailures
func Load() (ok bool) {
	if len(histPath) == 0 {
		panic("dir for history file must be specified")
	}
	_, err := os.Stat(histPath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Logf("%s does not exist, assuming new install", histPath)
			return true
		} else {
			log.Logf("error %s stat'ing %s", err, histPath)
			return false
		}
	}

	data, err := ioutil.ReadFile(histPath)
	if err != nil {
		log.Logf("error %s reading %s", err, histPath)
		return false
	}
	var content serializationFmt
	err = json.Unmarshal(data, &content)
	if err != nil {
		log.Logf("Error %s loading imaging history", err)
		futil.RenameUnique(histPath, histName+"_bad")
	} else {
		results = content.ImageResults
	}
	return
}

//Check returns false if too many failures are recorded for an image, true otherwise.
func Check(name string) (ok bool) {
	for _, img := range results {
		if img.Image == name {
			return img.ImagingFailures+img.BootFailures <= MaxFailuresPerImg
		}
	}
	return true
}

/* RecordBootState records boot status (success/failure, how badly failed).
If imgName is empty or otherwise doesn't appear valid, update stats for first entry.
*/
func RecordBootState(imgName string, success bool, severity uint, bTime time.Time, notes string) {
	Load()
	var result *ImageResult
	i := 0

	//find record, if it exists
	for i = range results {
		if results[i].Image == imgName {
			result = results[i]
			break
		}
	}
	if result == nil {
		//no matching record (?!) - does imgName seem valid?
		valid := strings.HasPrefix(imgName, strs.ImgPrefix())
		if valid || len(results) == 0 {
			//insert new record
			log.Logf("Adding new record for %s", imgName)
			result = new(ImageResult)
			result.Image = imgName
		} else {
			//imgName isn't valid. assume it's the first image in the list
			log.Logf("Assuming image '%s' is really %s", imgName, results[0].Image)
			result = results[0]
		}
	}
	result.BootAttempts++
	thisBoot := fmt.Sprintf("Boot @ %s, success: %t", bTime.Format(time.RFC3339), success)
	if !success {
		if severity < 1 {
			severity = 1
		}
		result.BootFailures += severity
		thisBoot += fmt.Sprintf(", severity: %d, notes: %s", severity, notes)
	}
	result.Notes = append(result.Notes, thisBoot)
	results.moveOrAddFront(result)

	write(results)
}

func write(res ResultList) {
	var content serializationFmt
	content.ImageResults = res
	data, err := json.Marshal(content)
	if err != nil {
		log.Logf("error %s marshalling json for %v", err, content)
		return
	}
	err = ioutil.WriteFile(histPath, data, 0644)
	if err != nil {
		log.Logf("error %s writing data to %s", err, histPath)
	}
}

/* When added to log.Preboot, RebootHook updates the history file at the end of factory restore.
Updated entry is shuffled to front of list, so RecordBootState can determine which
entry to update even if the disktag is missing.
*/
func RebootHook(success bool) {
	chosenImage := dt.Get()
	if chosenImage == "" {
		chosenImage = "ERROR UNKNOWN IMAGE"
		log.Logf("RebootHook: error: image name unset")
	} else {
		log.Logf("Adding to history file: img=%s success=%t", chosenImage, success)
	}
	var img *ImageResult
	for idx := range results {
		if results[idx].Image == chosenImage {
			img = results[idx]
		}
	}
	if img == nil {
		//no records for this image
		img = &ImageResult{
			Image: chosenImage,
		}
	}
	img.ImagingAttempts++
	if !success {
		img.ImagingFailures++
	}
	note := fmt.Sprintf("Imaging @ %s, success: %t", time.Now().Format(time.RFC3339), success)

	img.Notes = append(img.Notes, note)
	results.moveOrAddFront(img)
	write(results)
}

//if item exists in list, make it the first item. otherwise insert as first item.
func (rl *ResultList) moveOrAddFront(item *ImageResult) {
	for i := range *rl {
		if (*rl)[i] == item {
			//it exists. delete, preserving order (and allowing GC of unused element)
			copy((*rl)[i:], (*rl)[i+1:])
			(*rl)[len(*rl)-1] = nil
			(*rl) = (*rl)[:len(*rl)-1]
			break
		}
	}
	//insert at front
	l := &ResultList{item}
	*rl = append(*l, (*rl)...)
}
