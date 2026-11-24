// Copyright GoFrame Author(https://goframe.org). All Rights Reserved.
//
// This Source Code Form is subject to the terms of the MIT License.
// If a copy of the MIT License was not distributed with this file,
// You can obtain one at https://github.com/gogf/gf.

package gjson

import (
	"strconv"
	"strings"

	"github.com/gogf/gf/v2/text/gstr"
)

// path is a pattern split into segments for hierarchical data access.
type path struct {
	segments []string
}

// parsePath splits a pattern using the separator char.
func parsePath(pattern string, separator byte) path {
	return path{
		segments: strings.Split(pattern, string(separator)),
	}
}

// size returns the segment count.
func (p path) size() int {
	return len(p.segments)
}

// segment returns the segment at the given index.
func (p path) segment(index int) string {
	return p.segments[index]
}

// segmentArrayIndex returns the array index encoded in a pattern segment.
// It returns false when the segment is not a non-negative integer.
func segmentArrayIndex(segment string) (int, bool) {
	if !gstr.IsNumeric(segment) {
		return 0, false
	}
	n, err := strconv.Atoi(segment)
	if err != nil || n < 0 {
		return 0, false
	}
	return n, true
}
