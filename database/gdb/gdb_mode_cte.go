// Copyright GoFrame Author(https://goframe.org). All Rights Reserved.
//
// This Source Code Form is subject to the terms of the MIT License.
// If a copy of the MIT was not distributed with this file,
// You can obtain one at https://github.com/gogf/gf.

package gdb

import (
	"bytes"
)

// cteItem defines the structure for a Common Table Expression (CTE) item.
// This structure is used internally to manage information about CTEs.
type cteItem struct {
	Name       string
	Recursive  bool
	CteSql     string
	CteSqlArgs []interface{}
}

// WithCte adds a Common Table Expression (CTE) to the model.
// This function allows defining a temporary result set that can be referenced within the query, optionally recursively.
// Parameters:
//
//	name: The name of the CTE, used to reference it within the SQL query.
//	query: The sub-query that defines the CTE.
//	recursive: Optional flag indicating whether the CTE should be recursive (default is false).
//
// Returns:
//
//	A new *Model instance, allowing for method chaining or further configuration.
func (m *Model) WithCte(name string, query *Model, recursive ...bool) *Model {
	model := m.getModel()
	if name == "" || query == nil {
		return model
	}
	r := false
	if len(recursive) > 0 {
		r = recursive[0]
	}
	if model.cteItems == nil {
		model.cteItems = make([]cteItem, 0)
	}
	ctx := model.db.GetCtx()
	cteSql, cteSqlArgs := query.getFormattedSqlAndArgs(ctx, SelectTypeDefault, false)
	cte := cteItem{
		Name:       name,
		CteSql:     cteSql,
		CteSqlArgs: cteSqlArgs,
		Recursive:  r,
	}
	model.cteItems = append(model.cteItems, cte)
	return model
}

// formatCte generates the SQL string and associated arguments for Common Table Expressions (CTEs).
// If there are no CTE items, it returns an empty string and nil arguments.
//
// Returns:
//   - string: The formatted CTE SQL string.
//   - []interface{}: The arguments corresponding to placeholders in the SQL string.
func (m *Model) formatCte() (string, []interface{}) {
	if m.cteItems == nil || len(m.cteItems) == 0 {
		return "", nil
	}
	var (
		builder bytes.Buffer
		size    = len(m.cteItems)
		r       = false
		args    = make([]interface{}, 0)
	)
	builder.WriteString("WITH ")
	for i := range m.cteItems {
		if m.cteItems[i].Recursive {
			r = true
		}
	}
	if r {
		builder.WriteString("RECURSIVE ")
	}
	for k, v := range m.cteItems {
		builder.WriteString(m.QuoteWord(v.Name))
		builder.WriteString(" AS (")
		builder.WriteString(v.CteSql)
		builder.WriteString(")")
		if k != size-1 {
			builder.WriteString(",")
		}
		args = append(args, v.CteSqlArgs...)
	}
	return builder.String(), args
}
