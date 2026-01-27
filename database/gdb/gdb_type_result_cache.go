// Copyright GoFrame Author(https://goframe.org). All Rights Reserved.
//
// This Source Code Form is subject to the terms of the MIT License.
// If a copy of the MIT was not distributed with this file,
// You can obtain one at https://github.com/gogf/gf.

package gdb

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/gogf/gf/v2/container/gmap"
	"github.com/gogf/gf/v2/container/gtype"
)

var typeFieldCacheChecker = func(v *fieldCacheItem) bool { return v == nil }

type typeFieldCacheManager struct {
	cache *gmap.KVMap[typeFieldCacheKey, *fieldCacheItem]
}

type typeFieldCacheKey struct {
	typ              reflect.Type
	bindToAttrName   string
	relationAttrName string
}

func newTypeFieldCacheManager() *typeFieldCacheManager {
	return &typeFieldCacheManager{
		cache: gmap.NewKVMapWithChecker[typeFieldCacheKey, *fieldCacheItem](typeFieldCacheChecker, true),
	}
}

var fieldCacheInstance = newTypeFieldCacheManager()

type fieldCacheItem struct {
	// Deterministic field index (can be safely cached)
	bindToAttrIndex       int          // Field index of bound attribute (e.g. UserDetail), -1 for embedded fields
	bindToAttrIndexPath   []int        // Full index path for embedded fields (e.g. []int{1, 2})
	relationAttrIndex     int          // Field index of relation attribute (e.g. User, -1 means none)
	relationAttrIndexPath []int        // Full index path for embedded relation attribute
	isPointerElem         bool         // Whether array element is pointer type
	bindToAttrKind        reflect.Kind // Type of bound attribute

	// Field name mapping (supports case-insensitive lookup)
	fieldNameMap  map[string]string // lowercase -> OriginalName
	fieldIndexMap map[string]int    // FieldName -> Index
}

func (m *typeFieldCacheManager) getOrSet(
	arrayItemType reflect.Type,
	bindToAttrName string,
	relationAttrName string,
) (*fieldCacheItem, error) {
	e := gtype.New(nil, true)
	cacheKey := typeFieldCacheKey{
		typ:              arrayItemType,
		bindToAttrName:   bindToAttrName,
		relationAttrName: relationAttrName,
	}
	v := m.cache.GetOrSetFuncLock(cacheKey, func() *fieldCacheItem {
		item, err := m.buildCacheItem(arrayItemType, bindToAttrName, relationAttrName)
		if err != nil {
			e.Set(err)
			return nil
		}
		return item
	})
	if e.Val() != nil {
		return nil, e.Val().(error)
	}
	return v, nil
}

func (m *typeFieldCacheManager) buildCacheItem(
	arrayItemType reflect.Type,
	bindToAttrName string,
	relationAttrName string,
) (*fieldCacheItem, error) {
	structType := arrayItemType
	isPointerElem := false
	if structType.Kind() == reflect.Pointer {
		structType = structType.Elem()
		isPointerElem = true
	}

	if structType.Kind() != reflect.Struct {
		return nil, fmt.Errorf("arrayItemType must be struct or pointer to struct, got: %s", arrayItemType.Kind())
	}

	numField := structType.NumField()
	cache := &fieldCacheItem{
		relationAttrIndex: -1,
		isPointerElem:     isPointerElem,
		fieldNameMap:      make(map[string]string, numField),
		fieldIndexMap:     make(map[string]int, numField),
	}

	// Iterate all fields, build field mapping
	for i := 0; i < numField; i++ {
		field := structType.Field(i)
		fieldName := field.Name

		cache.fieldIndexMap[fieldName] = i
		cache.fieldNameMap[strings.ToLower(fieldName)] = fieldName
	}

	// Find bindToAttrName field index
	cache.bindToAttrIndex, cache.bindToAttrIndexPath = m.findFieldIndex(structType, cache.fieldIndexMap, cache.fieldNameMap, bindToAttrName)
	if cache.bindToAttrIndex < 0 && len(cache.bindToAttrIndexPath) == 0 {
		return nil, fmt.Errorf(`field "%s" not found in type %s`, bindToAttrName, arrayItemType.String())
	}

	// Set bindToAttrKind based on the found field
	var bindToField reflect.StructField
	if cache.bindToAttrIndex >= 0 {
		bindToField = structType.Field(cache.bindToAttrIndex)
	} else {
		bindToField = structType.FieldByIndex(cache.bindToAttrIndexPath)
	}
	cache.bindToAttrKind = bindToField.Type.Kind()

	// Find relationAttrName field index (optional)
	if relationAttrName != "" {
		cache.relationAttrIndex, cache.relationAttrIndexPath = m.findFieldIndex(structType, cache.fieldIndexMap, cache.fieldNameMap, relationAttrName)
		// Note: if still not found, keep -1, indicating that arrayElemValue itself should be used
	}

	return cache, nil
}

// findFieldIndex finds the field index or index path for a given field name
// Returns: index (>=0 for direct field, -1 for embedded field), indexPath (nil for direct field, actual path for embedded field)
func (m *typeFieldCacheManager) findFieldIndex(
	structType reflect.Type,
	fieldIndexMap map[string]int,
	fieldNameMap map[string]string,
	fieldName string,
) (int, []int) {
	// First, try direct lookup
	if idx, ok := fieldIndexMap[fieldName]; ok {
		return idx, nil
	}

	// Second, try case-insensitive lookup
	lowerFieldName := strings.ToLower(fieldName)
	if originalName, ok := fieldNameMap[lowerFieldName]; ok {
		return fieldIndexMap[originalName], nil
	}

	// Third, try using FieldByName for embedded fields
	field, ok := structType.FieldByName(fieldName)
	if !ok {
		return -1, nil // Not found
	}

	// For embedded fields, field.Index contains the full path
	if len(field.Index) == 1 {
		// Direct field (shouldn't normally happen here since we already checked fieldIndexMap)
		return field.Index[0], nil
	} else {
		// Embedded field - return the full index path
		return -1, field.Index
	}
}

// clear clears all cache (used for testing or hot updates)
func (m *typeFieldCacheManager) clear() {
	m.cache.Clear()
}

// ClearFieldCache clears field cache (for external calls)
// Used for testing or application hot update scenarios
func ClearFieldCache() {
	fieldCacheInstance.clear()
}
