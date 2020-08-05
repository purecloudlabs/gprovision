// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package log

import (
	"fmt"
)

var attrs map[string]interface{} = map[string]interface{}{}
var EAttrExists = fmt.Errorf("An attr with this name already exists")

// Get an attribute of the current log stack. Newly-attached logs must register
// any attrs with unique names.
func GetAttr(key string) (interface{}, bool) {
	v, ok := attrs[key]
	return v, ok
}

// Set an attribute of the current log stack. Newly-attached logs must register
// any attrs with unique names.
func SetAttr(key string, val interface{}) error {
	_, exists := attrs[key]
	if exists {
		return EAttrExists
	}
	attrs[key] = val
	return nil
}

//Remove all attrs from the map
func ClearAttrs() {
	for key := range attrs {
		delete(attrs, key)
	}
}
