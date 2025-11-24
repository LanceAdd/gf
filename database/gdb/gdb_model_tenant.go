package gdb

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/gogf/gf/v2/container/garray"
	"github.com/gogf/gf/v2/internal/reflection"
	"github.com/gogf/gf/v2/text/gregex"
	"github.com/gogf/gf/v2/text/gstr"
	"github.com/gogf/gf/v2/util/gutil"
)

type TenantValueType string

const (
	ArrayOrSliceType TenantValueType = "ArrayOrSlice"
	BaseType         TenantValueType = "BaseType"
	NilType          TenantValueType = "Nil"
)
const (
	CtxKeyForTenantIdField = "CtxKeyForTenantIdField"
	CtxKeyForTenantIdValue = "CtxKeyForTenantIdValue"
)

func WithTenantIdField(ctx context.Context, field string) context.Context {
	return context.WithValue(ctx, CtxKeyForTenantIdField, field)
}
func WithTenantIdValue(ctx context.Context, value any) context.Context {
	return context.WithValue(ctx, CtxKeyForTenantIdValue, value)
}

func DefaultGetTenantIdFieldValue(ctx context.Context) (field string, value any) {
	value = ctx.Value(CtxKeyForTenantIdValue)
	if f := ctx.Value(CtxKeyForTenantIdField); f != nil {
		if a, ok := f.(string); ok {
			return a, value
		}
	}
	return
}

// TenantOption provides configuration options for multi-tenancy support.
type TenantOption struct {
	Enable                    bool                                                // Enable controls whether the tenant feature is enabled.
	PropagateToJoins          bool                                                // PropagateToJoins determines whether tenant conditions should be applied to joined tables.
	GetTenantIdFieldValueFunc func(ctx context.Context) (field string, value any) // GetTenantIdFieldValueFunc is a function that retrieves the tenant ID field and value for a given context.
}

func (m *Model) Tenant(options ...TenantOption) *Model {
	model := m.getModel()
	if len(options) > 0 {
		model.tenantOption = options[0]
		return model
	}
	model.tenantOption.Enable = true
	return model
}

func (m *Model) tenantMaintainer() *TenantMaintainer {
	return &TenantMaintainer{
		Model: m,
	}
}

type TenantMaintainer struct {
	*Model
}

func (tm *TenantMaintainer) AppendTenantCondition(ctx context.Context) {
	if !tm.tenantOption.Enable {
		return
	}
	tenantCondition, tenantConditionArgs, tenantValueType := tm.tenantMaintainer().getWhereConditionForTenant(ctx)
	switch tenantValueType {
	case ArrayOrSliceType:
		tenantCondition.Iterator(func(k int, v string) bool {
			if value, found := tenantConditionArgs.Get(k); found {
				tm.WhereIn(v, value)
			}
			return true
		})
	case NilType:
		tenantCondition.Iterator(func(k int, v string) bool {
			tm.WhereNull(v)
			return true
		})
	case BaseType:
		tenantCondition.Iterator(func(k int, v string) bool {
			if value, found := tenantConditionArgs.Get(k); found {
				tm.Wheref(v, value)
			}
			return true
		})
	}
}

func (tm *TenantMaintainer) getWhereConditionForTenant(ctx context.Context) (*garray.StrArray, *garray.Array, TenantValueType) {
	var (
		tenantIdField string
		tenantIdValue any
	)
	if tm.tenantOption.GetTenantIdFieldValueFunc == nil {
		tenantIdField, tenantIdValue = DefaultGetTenantIdFieldValue(ctx)
	} else {
		tenantIdField, tenantIdValue = tm.tenantOption.GetTenantIdFieldValueFunc(ctx)
	}
	if tenantIdField == "" {
		return nil, nil, ""
	}
	conditionArray := garray.NewStrArray()
	argArray := garray.NewArray()
	tenantValueType := tm.getTenantValueType(tenantIdValue)
	if gstr.Contains(tm.tables, " JOIN ") {
		tableMatch, _ := gregex.MatchString(`(.+?) [A-Z]+ JOIN`, tm.tables)
		if c := tm.getConditionOfTableStringForTenant(ctx, tableMatch[1], tenantIdField, tenantValueType); c != "" {
			conditionArray.Append(c)
			if tenantValueType != NilType {
				argArray.Append(tenantIdValue)
			}
		}
		if tm.tenantOption.PropagateToJoins {
			tableMatches, _ := gregex.MatchAllString(`JOIN ([^()]+?) ON`, tm.tables)
			for _, match := range tableMatches {
				if c := tm.getConditionOfTableStringForTenant(ctx, match[1], tenantIdField, tenantValueType); c != "" {
					conditionArray.Append(c)
					if tenantValueType != NilType {
						argArray.Append(tenantIdValue)
					}
				}
			}
		}
	}
	if conditionArray.Len() == 0 && gstr.Contains(tm.tables, ",") {
		for _, s := range gstr.SplitAndTrim(tm.tables, ",") {
			if c := tm.getConditionOfTableStringForTenant(ctx, s, tenantIdField, tenantValueType); c != "" {
				conditionArray.Append(c)
				if tenantValueType != NilType {
					argArray.Append(tenantIdValue)
				}
			}
		}
	}
	conditionArray.FilterEmpty()
	return conditionArray, argArray, tenantValueType
}

func (tm *TenantMaintainer) getTenantValueType(value any) TenantValueType {
	if value == nil {
		return NilType
	}
	reflectInfo := reflection.OriginValueAndKind(value)
	switch reflectInfo.OriginKind {
	case reflect.Array, reflect.Slice:
		return ArrayOrSliceType
	default:
		return BaseType
	}
}

func (tm *TenantMaintainer) getConditionOfTableStringForTenant(ctx context.Context, s string, tenantIdField string, t TenantValueType) string {
	var (
		table  string
		schema string
		array1 = gstr.SplitAndTrim(s, " ")
		array2 = gstr.SplitAndTrim(array1[0], ".")
	)
	if len(array2) >= 2 {
		table = array2[1]
		schema = array2[0]
	} else {
		table = array2[0]
	}
	if !tm.existFieldName(ctx, schema, table, tenantIdField) {
		return ""
	}
	if len(array1) >= 3 {
		return tm.getConditionByFieldAndValue(array1[2], tenantIdField, t)
	}
	if len(array1) >= 2 {
		return tm.getConditionByFieldAndValue(array1[1], tenantIdField, t)
	}
	return tm.getConditionByFieldAndValue(table, tenantIdField, t)

}

func (tm *TenantMaintainer) getConditionByFieldAndValue(fieldPrefix, fieldName string, t TenantValueType) string {
	var (
		quotedFieldPrefix = tm.db.GetCore().QuoteWord(fieldPrefix)
		quotedFieldName   = tm.db.GetCore().QuoteWord(fieldName)
	)
	if quotedFieldPrefix != "" {
		quotedFieldName = fmt.Sprintf(`%s.%s`, quotedFieldPrefix, quotedFieldName)
	}
	switch t {
	case BaseType:
		return fmt.Sprintf(`%s = ?`, quotedFieldName)
	default:
		return quotedFieldName
	}
}

func (tm *TenantMaintainer) existFieldName(ctx context.Context, schema string, table string, tenantIdField string) bool {
	group := tm.db.GetGroup()
	key := genTableFieldsCacheKey(group, gutil.GetOrDefaultStr(tm.db.GetSchema(), schema), strings.Trim(table, "`"))
	v, err := tm.db.GetCore().GetInnerMemCache().Get(ctx, key)
	if err != nil {
		return false
	}
	if !v.IsNil() {
		if fields, ok := v.Val().(map[string]*TableField); ok {
			if _, ok := fields[tenantIdField]; ok {
				return true
			}
		}
	}
	return false
}
