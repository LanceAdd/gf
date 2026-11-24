// Copyright GoFrame Author(https://goframe.org). All Rights Reserved.
//
// This Source Code Form is subject to the terms of the MIT License.
// If a copy of the MIT License was not distributed with this file,
// You can obtain one at https://github.com/gogf/gf.

package gjson

import (
	"strconv"
	"strings"

	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"
)

// path is a pattern split into segments for hierarchical data access.
type path struct {
	segments []string
}

// parsePath splits a pattern using the separator char.
//
// The separator and the backslash can be escaped with a backslash.
// Empty segments are invalid.
func parsePath(pattern string, separator byte) (path, error) {
	if pattern == "" {
		return path{}, gerror.NewCode(gcode.CodeInvalidParameter, `invalid empty pattern`)
	}
	if separator == '\\' || !strings.ContainsRune(pattern, '\\') {
		segments := strings.Split(pattern, string(separator))
		for _, segment := range segments {
			if segment == "" {
				return path{}, gerror.NewCodef(gcode.CodeInvalidParameter, `invalid empty segment in pattern "%s"`, pattern)
			}
		}
		return path{segments: segments}, nil
	}
	var (
		segments []string
		segment  strings.Builder
		escaped  bool
	)
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		if escaped {
			if c != separator && c != '\\' {
				return path{}, gerror.NewCodef(gcode.CodeInvalidParameter, `invalid escape "\%c" in pattern "%s"`, c, pattern)
			}
			segment.WriteByte(c)
			escaped = false
			continue
		}
		switch c {
		case '\\':
			escaped = true
		case separator:
			if segment.Len() == 0 {
				return path{}, gerror.NewCodef(gcode.CodeInvalidParameter, `invalid empty segment in pattern "%s"`, pattern)
			}
			segments = append(segments, segment.String())
			segment.Reset()
		default:
			segment.WriteByte(c)
		}
	}
	if escaped {
		return path{}, gerror.NewCodef(gcode.CodeInvalidParameter, `invalid trailing escape in pattern "%s"`, pattern)
	}
	if segment.Len() == 0 {
		return path{}, gerror.NewCodef(gcode.CodeInvalidParameter, `invalid empty segment in pattern "%s"`, pattern)
	}
	segments = append(segments, segment.String())
	return path{segments: segments}, nil
}

// size returns the segment count.
func (p path) size() int {
	return len(p.segments)
}

// segment returns the segment at the given index.
func (p path) segment(index int) string {
	return p.segments[index]
}

// slice returns the sub-path starting at the given segment index.
func (p path) slice(from int) path {
	return path{
		segments: p.segments[from:],
	}
}

// hasEscape reports whether the pattern uses escapes.
func hasEscape(pattern string, separator byte) bool {
	return separator != '\\' && strings.ContainsRune(pattern, '\\')
}

// isDigits reports whether the segment contains only decimal digits.
func isDigits(segment string) bool {
	if segment == "" {
		return false
	}
	for i := 0; i < len(segment); i++ {
		if segment[i] < '0' || segment[i] > '9' {
			return false
		}
	}
	return true
}

// segmentIndex parses a decimal array index from a pattern segment.
// It returns false when the segment is not a decimal number that fits int.
func segmentIndex(segment string) (int, bool) {
	if !isDigits(segment) {
		return 0, false
	}
	n, err := strconv.Atoi(segment)
	if err != nil {
		return 0, false
	}
	return n, true
}
