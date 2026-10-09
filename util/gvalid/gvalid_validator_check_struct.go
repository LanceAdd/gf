// Copyright GoFrame Author(https://goframe.org). All Rights Reserved.
//
// This Source Code Form is subject to the terms of the MIT License.
// If a copy of the MIT was not distributed with this file,
// You can obtain one at https://github.com/gogf/gf.

// Struct validation collects field metadata, merges struct tags with external
// rules, checks nested values, and runs field rules in sequence.

package gvalid

import (
	"context"
	"reflect"
	"strings"

	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/internal/empty"
	"github.com/gogf/gf/v2/os/gstructs"
	"github.com/gogf/gf/v2/util/gconv"
	"github.com/gogf/gf/v2/util/gmeta"
	"github.com/gogf/gf/v2/util/gutil"
)

// structCheck is the working state of one doCheckStruct call.
// Phases mutate it in order. The field comments mark state that must stay shared.
type structCheck struct {
	// object is the struct value passed to doCheckStruct.
	object any
	// errorMaps stores field name to rule name to validation error.
	errorMaps map[string]map[string]error
	// fieldToAliasNameMap maps a Go field name to the alias used in validation.
	fieldToAliasNameMap map[string]string
	// resultSequenceRules collects rules discovered by recursive value checks.
	resultSequenceRules []fieldRule
	// fieldMap is the struct field map keyed by alias tag or field name.
	// Iteration order is map order and must stay unsorted.
	fieldMap map[string]gstructs.Field
	// tagFields is the ordered list of fields that carry a validation tag.
	tagFields []gstructs.Field
	// inputParamMap is the value map read by rules and nested checks.
	inputParamMap map[string]any
	// checkRules is the ordered field rules to execute.
	checkRules []fieldRule
	// nameToRuleMap indexes rules by validation name.
	// When v.rules is map[string]string, this map is that same map.
	nameToRuleMap map[string]string
	// customMessage stores field custom error messages.
	customMessage CustomMsg
	// checkValueData is the raw data passed to single-value checks.
	checkValueData any
	// value is reused by nested field checks and the later rule loop.
	// A gmeta.Meta rule does not refresh it.
	value any
}

// doCheckStruct validates struct object with rules from struct tags and Validator.Rules.
func (v *Validator) doCheckStruct(ctx context.Context, object any) Error {
	check, err := v.prepareStructCheck(object)
	if err != nil || check == nil {
		return err
	}
	v.mergeStructTags(check)
	v.overlayStructMessages(check)
	if err = v.checkStructFieldsRecursively(ctx, check); err != nil {
		return err
	}
	return v.runStructFieldRules(ctx, check)
}

// prepareStructCheck loads struct metadata, external rules, and the parameter map.
// A nil check with a nil error means validation has nothing to do.
func (v *Validator) prepareStructCheck(object any) (*structCheck, Error) {
	var (
		isEmptyData  = empty.IsEmpty(v.data)
		isEmptyAssoc = empty.IsEmpty(v.assoc)
	)
	fieldMap, err := gstructs.FieldMap(gstructs.FieldMapInput{
		Pointer:          object,
		PriorityTagArray: aliasNameTagPriority,
		RecursiveOption:  gstructs.RecursiveOptionEmbedded,
	})
	if err != nil {
		return nil, newValidationErrorByStr(internalObjectErrRuleName, err)
	}
	// It here must use gstructs.TagFields not gstructs.FieldMap to ensure error sequence.
	tagFields, err := gstructs.TagFields(object, structTagPriority)
	if err != nil {
		return nil, newValidationErrorByStr(internalObjectErrRuleName, err)
	}
	// If there's no struct tag and validation rules, it does nothing and returns quickly.
	if len(tagFields) == 0 && v.messages == nil && isEmptyData && isEmptyAssoc {
		return nil, nil
	}

	check := &structCheck{
		object:              object,
		errorMaps:           make(map[string]map[string]error),
		fieldToAliasNameMap: make(map[string]string),
		resultSequenceRules: make([]fieldRule, 0),
		fieldMap:            fieldMap,
		tagFields:           tagFields,
		checkRules:          make([]fieldRule, 0),
		nameToRuleMap:       make(map[string]string),
		customMessage:       make(CustomMsg),
	}
	// checkValueData falls back to the struct when no associated data is set.
	check.checkValueData = v.assoc
	if check.checkValueData == nil {
		check.checkValueData = object
	}
	switch assertValue := v.rules.(type) {
	// Sequence tag: []sequence tag
	// Sequence has order for error results.
	case []string:
		for _, tag := range assertValue {
			name, rule, msg := v.parseTagValue(tag)
			if len(name) == 0 {
				continue
			}
			appendRuleMessages(check.customMessage, name, rule, msg)
			check.nameToRuleMap[name] = rule
			check.checkRules = append(check.checkRules, fieldRule{
				Name: name,
				Rule: rule,
			})
		}

	// Map type rules does not support sequence.
	// Format: map[key]rule
	case map[string]string:
		// Keep the caller's map. Later alias renaming writes through to it.
		check.nameToRuleMap = assertValue
		for name, rule := range assertValue {
			check.checkRules = append(check.checkRules, fieldRule{
				Name: name,
				Rule: rule,
			})
		}
	}
	// If there's no struct tag and validation rules, it does nothing and returns quickly.
	if len(tagFields) == 0 && len(check.checkRules) == 0 && isEmptyData && isEmptyAssoc {
		return nil, nil
	}
	// Input parameter map handling.
	if v.assoc == nil || !v.useAssocInsteadOfObjectAttributes {
		check.inputParamMap = make(map[string]any)
	} else {
		check.inputParamMap = gconv.Map(v.assoc)
	}
	// Checks and extends the parameters map with struct alias tag.
	if !v.useAssocInsteadOfObjectAttributes {
		for nameOrTag, field := range fieldMap {
			check.inputParamMap[nameOrTag] = field.Value.Interface()
			if nameOrTag != field.Name() {
				check.inputParamMap[field.Name()] = field.Value.Interface()
			}
		}
	}
	return check, nil
}

// mergeStructTags merges validation tags into the external rules.
// An external rule for the same field overwrites the struct tag rule.
func (v *Validator) mergeStructTags(check *structCheck) {
	for _, field := range check.tagFields {
		v.applyStructTagField(check, field)
	}
}

// applyStructTagField merges one struct tag into check.
// It returns before reading the tag message when an external rule already uses the same name.
func (v *Validator) applyStructTagField(check *structCheck, field gstructs.Field) {
	var (
		isMeta          bool
		fieldName       = field.Name()                    // Attribute name.
		name, rule, msg = v.parseTagValue(field.TagValue) // The `name` is different from `attribute alias`, which is used for validation only.
	)
	if len(name) == 0 {
		if aliasName, ok := check.fieldToAliasNameMap[fieldName]; ok {
			// It uses alias name of the attribute if its alias name tag exists.
			name = aliasName
		} else {
			// It or else uses the attribute name directly.
			name = field.TagPriorityName()
		}
	} else {
		// It uses the alias name from validation rule.
		check.fieldToAliasNameMap[fieldName] = name
	}
	// It here extends the params map using alias names.
	// Note that the variable `name` might be alias name or attribute name.
	if _, ok := check.inputParamMap[name]; !ok {
		if !v.useAssocInsteadOfObjectAttributes {
			check.inputParamMap[name] = field.Value.Interface()
		} else if name != fieldName {
			if foundKey, foundValue := gutil.MapPossibleItemByKey(check.inputParamMap, fieldName); foundKey != "" {
				check.inputParamMap[name] = foundValue
			}
		}
	}

	if _, ok := check.nameToRuleMap[name]; !ok {
		if _, ok = check.nameToRuleMap[fieldName]; ok {
			// If there's alias name,
			// use alias name as its key and remove the field name key.
			check.nameToRuleMap[name] = check.nameToRuleMap[fieldName]
			delete(check.nameToRuleMap, fieldName)
			for index, checkRuleItem := range check.checkRules {
				if fieldName == checkRuleItem.Name {
					checkRuleItem.Name = name
					check.checkRules[index] = checkRuleItem
					break
				}
			}
		} else {
			check.nameToRuleMap[name] = rule
			if fieldValue := field.Value.Interface(); fieldValue != nil {
				_, isMeta = fieldValue.(gmeta.Meta)
			}
			check.checkRules = append(check.checkRules, fieldRule{
				Name:      name,
				Rule:      rule,
				IsMeta:    isMeta,
				FieldKind: field.OriginalKind(),
				FieldType: field.Type(),
			})
		}
	} else {
		// The input rules can overwrite the rules in struct tag.
		return
	}
	appendRuleMessages(check.customMessage, name, rule, msg)
}

// overlayStructMessages applies Validator.Messages on top of tag messages.
// A field alias replaces the original field name key.
func (v *Validator) overlayStructMessages(check *structCheck) {
	msg, ok := v.messages.(CustomMsg)
	if !ok || len(msg) == 0 {
		return
	}
	for key, msgName := range msg {
		if aliasName, ok := check.fieldToAliasNameMap[key]; ok {
			// Overwrite the key of field name.
			check.customMessage[aliasName] = msgName
		} else {
			check.customMessage[key] = msgName
		}
	}
}

// checkStructFieldsRecursively checks embedded structs and nested map, struct, slice, or array values.
// On bail, the returned error carries only the rules collected during recursion.
func (v *Validator) checkStructFieldsRecursively(ctx context.Context, check *structCheck) Error {
	for _, field := range check.fieldMap {
		// No validation interface implements check.
		if _, ok := field.Value.Interface().(iNoValidation); ok {
			continue
		}
		// No validation field tag check.
		if _, ok := field.TagLookup(noValidationTagName); ok {
			continue
		}
		if field.IsEmbedded() {
			// The attributes of embedded struct are considered as direct attributes of its parent struct.
			if err := v.doCheckStruct(ctx, field.Value); err != nil {
				// It merges the errors into single error map.
				for key, fieldErrors := range err.(*validationError).errors {
					check.errorMaps[key] = fieldErrors
				}
			}
		} else {
			// The `field.TagValue` is the alias name of field.Name().
			// Eg, value from struct tag `p`.
			if field.TagValue != "" {
				check.fieldToAliasNameMap[field.Name()] = field.TagValue
			}
			switch field.OriginalKind() {
			case reflect.Map, reflect.Struct, reflect.Slice, reflect.Array:
				// Recursively check attribute slice/map.
				check.value = getPossibleValueFromMap(
					check.inputParamMap, field.Name(), check.fieldToAliasNameMap[field.Name()],
				)
				if empty.IsNil(check.value) {
					switch field.Kind() {
					case reflect.Map, reflect.Pointer, reflect.Slice, reflect.Array:
						// Nothing to do.
						continue
					default:
					}
				}
				v.doCheckValueRecursively(ctx, doCheckValueRecursivelyInput{
					Value:               check.value,
					Kind:                field.OriginalKind(),
					Type:                field.Type().Type,
					ErrorMaps:           check.errorMaps,
					ResultSequenceRules: &check.resultSequenceRules,
				})
			default:
			}
		}
		if v.bail && len(check.errorMaps) > 0 {
			break
		}
	}
	if v.bail && len(check.errorMaps) > 0 {
		return newValidationError(gcode.CodeValidationFailed, check.resultSequenceRules, check.errorMaps)
	}
	return nil
}

// runStructFieldRules runs check.checkRules and returns the combined validation error.
// The result sequence is field rules first, then rules reported by nested checks.
func (v *Validator) runStructFieldRules(ctx context.Context, check *structCheck) Error {
	for _, checkRuleItem := range check.checkRules {
		// it ignores Meta object.
		if !checkRuleItem.IsMeta {
			check.value = getPossibleValueFromMap(
				check.inputParamMap, checkRuleItem.Name, check.fieldToAliasNameMap[checkRuleItem.Name],
			)
		}
		// Empty json string checks according to mapping field kind.
		if check.value != nil {
			switch checkRuleItem.FieldKind {
			case reflect.Struct, reflect.Map:
				// empty struct or map.
				if gconv.String(check.value) == emptyJsonObjectStr {
					check.value = nil
				}
			case reflect.Slice, reflect.Array:
				// empty slice.
				if gconv.String(check.value) == emptyJsonArrayStr {
					check.value = []any{}
				}
			default:
			}
		}
		// It checks each rule and its value in loop.
		if validatedError := v.doCheckValue(ctx, doCheckValueInput{
			Name:      checkRuleItem.Name,
			Value:     check.value,
			ValueType: checkRuleItem.FieldType,
			Rule:      checkRuleItem.Rule,
			Messages:  check.customMessage[checkRuleItem.Name],
			DataRaw:   check.checkValueData,
			DataMap:   check.inputParamMap,
		}); validatedError != nil {
			_, errorItem := validatedError.FirstItem()
			// ============================================================
			// Only in map and struct validations:
			// If value is nil or empty string and has no required* rules,
			// it clears the error message.
			// ============================================================
			if !checkRuleItem.IsMeta && (check.value == nil || gconv.String(check.value) == "") {
				required := false
				// rule => error
				for ruleKey := range errorItem {
					// it checks whether current rule is kind of required rule.
					if required = v.checkRuleRequired(ruleKey); required {
						break
					}
				}
				if !required {
					continue
				}
			}
			if _, ok := check.errorMaps[checkRuleItem.Name]; !ok {
				check.errorMaps[checkRuleItem.Name] = make(map[string]error)
			}
			for ruleKey, errorItemMsgMap := range errorItem {
				check.errorMaps[checkRuleItem.Name][ruleKey] = errorItemMsgMap
			}
			// Bail feature.
			if v.bail {
				break
			}
		}
	}
	if len(check.errorMaps) > 0 {
		return newValidationError(
			gcode.CodeValidationFailed,
			append(check.checkRules, check.resultSequenceRules...),
			check.errorMaps,
		)
	}
	return nil
}

// appendRuleMessages aligns custom messages with rules split by '|'.
// A message shorter than the rule list leaves the remaining rules on the default message.
// customMessage must be non-nil. Struct and map validation both use this alignment.
func appendRuleMessages(customMessage CustomMsg, name, rule, msg string) {
	if len(msg) == 0 {
		return
	}
	var (
		msgArray  = strings.Split(msg, "|")
		ruleArray = strings.Split(rule, "|")
	)
	for k, ruleKey := range ruleArray {
		// If length of custom messages is lesser than length of rules,
		// the rest rules use the default error messages.
		if len(msgArray) <= k {
			continue
		}
		if len(msgArray[k]) == 0 {
			continue
		}
		array := strings.Split(ruleKey, ":")
		if _, ok := customMessage[name]; !ok {
			customMessage[name] = make(map[string]string)
		}
		customMessage[name].(map[string]string)[strings.TrimSpace(array[0])] = strings.TrimSpace(msgArray[k])
	}
}

// getPossibleValueFromMap reads fieldName from inputParamMap, then aliasName when the field value is nil.
func getPossibleValueFromMap(inputParamMap map[string]any, fieldName, aliasName string) (value any) {
	_, value = gutil.MapPossibleItemByKey(inputParamMap, fieldName)
	if value == nil && aliasName != "" {
		_, value = gutil.MapPossibleItemByKey(inputParamMap, aliasName)
	}
	return
}
