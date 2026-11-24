// Copyright GoFrame Author(https://goframe.org). All Rights Reserved.
//
// This Source Code Form is subject to the terms of the MIT License.
// If a copy of the MIT was not distributed with this file,
// You can obtain one at https://github.com/gogf/gf.

// Package gjson provides convenient API for JSON/XML/INI/YAML/TOML data handling.
package gjson

import (
	"reflect"

	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/internal/reflection"
	"github.com/gogf/gf/v2/internal/rwmutex"
	"github.com/gogf/gf/v2/internal/utils"
	"github.com/gogf/gf/v2/util/gconv"
)

type ContentType = string

const (
	ContentTypeJSON       ContentType = `json`
	ContentTypeJs         ContentType = `js`
	ContentTypeXML        ContentType = `xml`
	ContentTypeIni        ContentType = `ini`
	ContentTypeYaml       ContentType = `yaml`
	ContentTypeYml        ContentType = `yml`
	ContentTypeToml       ContentType = `toml`
	ContentTypeProperties ContentType = `properties`
)

const (
	// Separator char for hierarchical data access.
	defaultSplitChar = '.'
)

// Json is the customized JSON struct.
type Json struct {
	mu rwmutex.RWMutex

	// Pointer for hierarchical data access, it's the root of data in default.
	p *any

	// Char separator('.' in default).
	c byte

	// Violence Check(false in default),
	// which is used to access data when the hierarchical data key contains separator char.
	vc bool
}

// Options for Json object creating/loading.
type Options struct {
	// Mark this object is for in concurrent-safe usage. This is especially for Json object creating.
	Safe bool

	// Custom priority tags for decoding, eg: "json,yaml,MyTag".
	// This is specially for struct parsing into Json object.
	Tags string

	// Type specifies the data content type, eg: json, xml, yaml, toml, ini.
	Type ContentType

	// StrNumber causes the Decoder to unmarshal a number into an any as a string instead of as a float64.
	// This is specially for json content parsing into Json object.
	StrNumber bool
}

// iInterfaces is used for type assert api for Interfaces().
type iInterfaces interface {
	Interfaces() []any
}

// iMapStrAny is the interface support for converting struct parameter to map.
type iMapStrAny interface {
	MapStrAny() map[string]any
}

// iVal is the interface for underlying any retrieving.
type iVal interface {
	Val() any
}

// setValue sets `value` to `j` by `pattern`.
// Note:
// 1. If value is nil and removed is true, means deleting this value;
// 2. It's quite complicated in hierarchical data search, node creating and data assignment;
func (j *Json) setValue(pattern string, value any, removed bool) error {
	var err error
	if value, err = j.convertValue(value); err != nil {
		return err
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.setValueLocked(pattern, value, removed)
}

// setValueLocked sets an already converted value, the caller must hold j.mu.
func (j *Json) setValueLocked(pattern string, value any, removed bool) error {
	segments, err := parsePath(pattern, j.c)
	if err != nil {
		return err
	}
	args := &setPathArgs{
		pattern:  pattern,
		value:    value,
		removing: removed && value == nil,
	}
	newRoot, err := j.setPath(*j.p, segments, args)
	if err != nil {
		return err
	}
	*j.p = newRoot
	return nil
}

// setPathArgs holds the state shared by one Set or Remove operation.
type setPathArgs struct {
	pattern  string
	value    any
	removing bool
}

// setPath resolves the remaining pattern segments and returns the new node value.
// It updates the current node only after the child node is resolved, so an error
// leaves the data unchanged.
func (j *Json) setPath(node any, segments path, args *setPathArgs) (any, error) {
	var (
		segment = segments.segment(0)
		rest    = segments.slice(1)
	)
	switch v := node.(type) {
	case map[string]any:
		return j.setMapPath(v, segment, rest, args)
	case []any:
		return j.setArrayPath(v, segment, rest, args)
	default:
		return j.setDefaultPath(node, segment, rest, args)
	}
}

// setMapPath resolves one pattern segment against a map node.
func (j *Json) setMapPath(node map[string]any, segment string, rest path, args *setPathArgs) (any, error) {
	if rest.size() == 0 {
		if args.removing {
			delete(node, segment)
			return node, nil
		}
		if node == nil {
			node = make(map[string]any)
		}
		node[segment] = args.value
		return node, nil
	}
	child, ok := node[segment]
	if !ok {
		if args.removing {
			return node, nil
		}
		var err error
		if child, err = newSetNode(rest.segment(0), args.pattern); err != nil {
			return nil, err
		}
	}
	newChild, err := j.setPath(child, rest, args)
	if err != nil {
		return nil, err
	}
	if node == nil {
		node = make(map[string]any)
	}
	node[segment] = newChild
	return node, nil
}

// setArrayPath resolves one pattern segment against an array node.
func (j *Json) setArrayPath(node []any, segment string, rest path, args *setPathArgs) (any, error) {
	n, ok := segmentIndex(segment)
	if !ok {
		return nil, newInvalidArrayIndexError(segment, args.pattern)
	}
	if rest.size() == 0 {
		if args.removing {
			if n < len(node) {
				return append(node[:n], node[n+1:]...), nil
			}
			return node, nil
		}
		if n < len(node) {
			node[n] = args.value
			return node, nil
		}
		if n == len(node) {
			return append(node, args.value), nil
		}
		return nil, newArrayIndexOutOfRangeError(segment, args.pattern)
	}
	if n < len(node) {
		newChild, err := j.setPath(node[n], rest, args)
		if err != nil {
			return nil, err
		}
		node[n] = newChild
		return node, nil
	}
	if args.removing {
		return node, nil
	}
	if n == len(node) {
		child, err := newSetNode(rest.segment(0), args.pattern)
		if err != nil {
			return nil, err
		}
		newChild, err := j.setPath(child, rest, args)
		if err != nil {
			return nil, err
		}
		return append(node, newChild), nil
	}
	return nil, newArrayIndexOutOfRangeError(segment, args.pattern)
}

// setDefaultPath replaces a scalar node with the hierarchy built from the remaining segments.
func (j *Json) setDefaultPath(node any, segment string, rest path, args *setPathArgs) (any, error) {
	if args.removing {
		return node, nil
	}
	if isDigits(segment) {
		n, ok := segmentIndex(segment)
		if !ok {
			return nil, newInvalidArrayIndexError(segment, args.pattern)
		}
		if n != 0 {
			return nil, newArrayIndexOutOfRangeError(segment, args.pattern)
		}
		nodeArray := make([]any, 1)
		if rest.size() == 0 {
			nodeArray[0] = args.value
			return nodeArray, nil
		}
		child, err := newSetNode(rest.segment(0), args.pattern)
		if err != nil {
			return nil, err
		}
		newChild, err := j.setPath(child, rest, args)
		if err != nil {
			return nil, err
		}
		nodeArray[0] = newChild
		return nodeArray, nil
	}
	if rest.size() == 0 {
		return map[string]any{segment: args.value}, nil
	}
	child, err := newSetNode(rest.segment(0), args.pattern)
	if err != nil {
		return nil, err
	}
	newChild, err := j.setPath(child, rest, args)
	if err != nil {
		return nil, err
	}
	return map[string]any{segment: newChild}, nil
}

// newSetNode creates an empty node matching the type implied by the next segment.
func newSetNode(segment, pattern string) (any, error) {
	if isDigits(segment) {
		if _, ok := segmentIndex(segment); !ok {
			return nil, newInvalidArrayIndexError(segment, pattern)
		}
		return make([]any, 0), nil
	}
	return map[string]any{}, nil
}

func newInvalidArrayIndexError(segment, pattern string) error {
	return gerror.NewCodef(gcode.CodeInvalidParameter, `invalid array index "%s" in pattern "%s"`, segment, pattern)
}

func newArrayIndexOutOfRangeError(segment, pattern string) error {
	return gerror.NewCodef(gcode.CodeInvalidParameter, `array index "%s" out of range in pattern "%s"`, segment, pattern)
}

func newInvalidVariableTypeError(pattern string) error {
	return gerror.NewCodef(gcode.CodeInvalidParameter, "invalid variable type of %s", pattern)
}

// appendValueLocked appends an already converted value, the caller must hold j.mu.
func (j *Json) appendValueLocked(pattern string, value any) error {
	if pattern == "." {
		node, err := appendToArray(*j.p, value, pattern)
		if err != nil {
			return err
		}
		*j.p = node
		return nil
	}
	segments, err := parsePath(pattern, j.c)
	if err != nil {
		return err
	}
	newRoot, err := j.appendPath(*j.p, segments, value, pattern)
	if err != nil {
		return err
	}
	*j.p = newRoot
	return nil
}

// appendPath resolves the target node and appends one element to it.
func (j *Json) appendPath(node any, segments path, value any, pattern string) (any, error) {
	if segments.size() == 0 {
		return appendToArray(node, value, pattern)
	}
	var (
		segment = segments.segment(0)
		rest    = segments.slice(1)
	)
	switch v := node.(type) {
	case map[string]any:
		if rest.size() == 0 {
			if v == nil {
				return map[string]any{segment: []any{value}}, nil
			}
			child, ok := v[segment]
			if !ok || child == nil {
				v[segment] = []any{value}
				return v, nil
			}
			array, ok := child.([]any)
			if !ok {
				return nil, newInvalidVariableTypeError(pattern)
			}
			v[segment] = append(array, value)
			return v, nil
		}
		child, ok := v[segment]
		if !ok {
			var err error
			if child, err = newSetNode(rest.segment(0), pattern); err != nil {
				return nil, err
			}
		}
		newChild, err := j.appendPath(child, rest, value, pattern)
		if err != nil {
			return nil, err
		}
		if v == nil {
			v = make(map[string]any)
		}
		v[segment] = newChild
		return v, nil
	case []any:
		n, ok := segmentIndex(segment)
		if !ok {
			return nil, newInvalidArrayIndexError(segment, pattern)
		}
		if rest.size() == 0 {
			if n < len(v) {
				return appendArrayElement(v, n, value, pattern)
			}
			if n == len(v) {
				return append(v, []any{value}), nil
			}
			return nil, newArrayIndexOutOfRangeError(segment, pattern)
		}
		if n < len(v) {
			newChild, err := j.appendPath(v[n], rest, value, pattern)
			if err != nil {
				return nil, err
			}
			v[n] = newChild
			return v, nil
		}
		if n == len(v) {
			child, err := newSetNode(rest.segment(0), pattern)
			if err != nil {
				return nil, err
			}
			newChild, err := j.appendPath(child, rest, value, pattern)
			if err != nil {
				return nil, err
			}
			return append(v, newChild), nil
		}
		return nil, newArrayIndexOutOfRangeError(segment, pattern)
	default:
		if isDigits(segment) {
			n, ok := segmentIndex(segment)
			if !ok {
				return nil, newInvalidArrayIndexError(segment, pattern)
			}
			if n != 0 {
				return nil, newArrayIndexOutOfRangeError(segment, pattern)
			}
			child, err := j.appendPath(nil, rest, value, pattern)
			if err != nil {
				return nil, err
			}
			return []any{child}, nil
		}
		child, err := j.appendPath(nil, rest, value, pattern)
		if err != nil {
			return nil, err
		}
		return map[string]any{segment: child}, nil
	}
}

// appendArrayElement appends value to the array stored at node[index].
func appendArrayElement(node []any, index int, value any, pattern string) (any, error) {
	child := node[index]
	if child == nil {
		node[index] = []any{value}
		return node, nil
	}
	array, ok := child.([]any)
	if !ok {
		return nil, newInvalidVariableTypeError(pattern)
	}
	node[index] = append(array, value)
	return node, nil
}

// appendToArray appends value to the given array node.
func appendToArray(node any, value any, pattern string) (any, error) {
	if node == nil {
		return []any{value}, nil
	}
	array, ok := node.([]any)
	if !ok {
		return nil, newInvalidVariableTypeError(pattern)
	}
	return append(array, value), nil
}

// convertValue converts `value` to map[string]any or []any,
// which can be supported for hierarchical data access.
func (j *Json) convertValue(value any) (convertedValue any, err error) {
	if value == nil {
		return
	}

	switch value.(type) {
	case map[string]any:
		return value, nil

	case []any:
		return value, nil

	default:
		var (
			reflectInfo = reflection.OriginValueAndKind(value)
		)
		switch reflectInfo.OriginKind {
		case reflect.Array:
			return gconv.Interfaces(value), nil

		case reflect.Slice:
			return gconv.Interfaces(value), nil

		case reflect.Map:
			return gconv.Map(value), nil

		case reflect.Struct:
			if v, ok := value.(iMapStrAny); ok {
				convertedValue = v.MapStrAny()
			}
			if utils.IsNil(convertedValue) {
				if v, ok := value.(iInterfaces); ok {
					convertedValue = v.Interfaces()
				}
			}
			if utils.IsNil(convertedValue) {
				convertedValue = gconv.Map(value)
			}
			if utils.IsNil(convertedValue) {
				err = gerror.NewCodef(gcode.CodeInvalidParameter, `unsupported value type "%s"`, reflect.TypeOf(value))
			}
			return

		default:
			return value, nil
		}
	}
}

// getPointerByPattern returns a pointer to the value by specified `pattern`.
func (j *Json) getPointerByPattern(pattern string) *any {
	if j.p == nil {
		return nil
	}
	if j.vc {
		return j.getPointerByPatternWithViolenceCheck(pattern)
	} else {
		return j.getPointerByPatternWithoutViolenceCheck(pattern)
	}
}

// getPointerByPatternWithViolenceCheck returns a pointer to the value of specified `pattern` with violence check.
func (j *Json) getPointerByPatternWithViolenceCheck(pattern string) *any {
	if !j.vc {
		return j.getPointerByPatternWithoutViolenceCheck(pattern)
	}

	// It returns nil if pattern is empty.
	if pattern == "" {
		return nil
	}
	// It returns all if pattern is ".".
	if pattern == "." {
		return j.p
	}

	if hasEscape(pattern, j.c) {
		return nil
	}
	segments, err := parsePath(pattern, j.c)
	if err != nil {
		return nil
	}
	var (
		pointer = j.p
		base    = 0
	)
	for i := 0; i < segments.size(); i++ {
		var (
			end     = len(pattern)
			matched = false
		)
		for k := segments.size(); k > i; k-- {
			// Try the longest remaining key first, then move the end
			// before the last separator.
			key := pattern[base:end]
			if key != "" {
				if r := j.checkPatternByPointer(key, pointer); r != nil {
					if k == segments.size() {
						return r
					}
					pointer = r
					base = end + 1
					i = k - 1
					matched = true
					break
				}
			}
			end -= len(segments.segment(k-1)) + 1
		}
		if !matched {
			return nil
		}
	}
	return nil
}

// getPointerByPatternWithoutViolenceCheck returns a pointer to the value of specified `pattern`, with no violence check.
func (j *Json) getPointerByPatternWithoutViolenceCheck(pattern string) *any {
	if j.vc {
		return j.getPointerByPatternWithViolenceCheck(pattern)
	}

	// It returns nil if pattern is empty.
	if pattern == "" {
		return nil
	}
	// It returns all if pattern is ".".
	if pattern == "." {
		return j.p
	}

	segments, err := parsePath(pattern, j.c)
	if err != nil {
		return nil
	}
	var (
		length  = segments.size()
		pointer = j.p
	)
	for k := 0; k < length; k++ {
		r := j.checkPatternByPointer(segments.segment(k), pointer)
		if r == nil {
			break
		}
		if k == length-1 {
			return r
		}
		pointer = r
	}
	return nil
}

// checkPatternByPointer checks whether there's value by `key` in specified `pointer`.
// It returns a pointer to the value.
func (j *Json) checkPatternByPointer(key string, pointer *any) *any {
	switch (*pointer).(type) {
	case map[string]any:
		if v, ok := (*pointer).(map[string]any)[key]; ok {
			return &v
		}
	case []any:
		if n, ok := segmentIndex(key); ok && len((*pointer).([]any)) > n {
			return &(*pointer).([]any)[n]
		}
	}
	return nil
}
