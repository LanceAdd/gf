// Copyright GoFrame gf Author(https://goframe.org). All Rights Reserved.
//
// This Source Code Form is subject to the terms of the MIT License.
// If a copy of the MIT was not distributed with this file,
// You can obtain one at https://github.com/gogf/gf.

package seata

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/os/glog"
	"github.com/gogf/gf/v2/util/gconv"
)

// TableRecords represents the table records for undo log.
// Compatible with Seata's TableRecords structure.
type TableRecords struct {
	TableName string `json:"tableName"`
	Rows      []Row  `json:"rows"`
}

// Row represents a single row in the table.
type Row struct {
	Fields []Field `json:"fields"`
}

// Field represents a field in a row.
type Field struct {
	Name    string      `json:"name"`
	KeyType int32       `json:"keyType"` // 0: common, 1: primary key
	Type    int32       `json:"type"`    // SQL type
	Value   interface{} `json:"value"`
}

const (
	// KeyTypeCommon represents a common field (not a primary key).
	KeyTypeCommon = 0
	// KeyTypePrimaryKey represents a primary key field.
	KeyTypePrimaryKey = 1
)

// ImageGenerator is responsible for generating before/after images.
type ImageGenerator struct {
	resource *Resource
}

// NewImageGenerator creates a new ImageGenerator.
func NewImageGenerator(resource *Resource) *ImageGenerator {
	return &ImageGenerator{
		resource: resource,
	}
}

// GenerateBeforeImage generates the before image for UPDATE/DELETE.
// It queries the rows that will be affected before the actual modification.
func (g *ImageGenerator) GenerateBeforeImage(
	ctx context.Context,
	link gdb.Link,
	tableName string,
	condition string,
	args []interface{},
) (*TableRecords, error) {
	// Build SELECT SQL with FOR UPDATE to lock the rows
	selectSQL := g.buildSelectSQL(tableName, condition)

	glog.Debugf(ctx, "[Seata] GenerateBeforeImage SQL: %s, args: %v", selectSQL, args)

	// Execute query
	result, err := g.executeQuery(ctx, link, selectSQL, args)
	if err != nil {
		return nil, gerror.Wrap(err, "failed to query before image")
	}

	// Convert to TableRecords
	return g.convertToTableRecords(ctx, tableName, result)
}

// GenerateAfterImage generates the after image for UPDATE/INSERT.
// It queries the rows after the modification using primary key values.
func (g *ImageGenerator) GenerateAfterImage(
	ctx context.Context,
	link gdb.Link,
	tableName string,
	pkValues []interface{},
) (*TableRecords, error) {
	if len(pkValues) == 0 {
		// No rows affected, return empty TableRecords
		return &TableRecords{
			TableName: tableName,
			Rows:      []Row{},
		}, nil
	}

	// Get primary key name
	pkName, err := g.getPrimaryKeyName(ctx, tableName)
	if err != nil {
		return nil, err
	}

	// Build SELECT SQL using primary keys
	selectSQL := g.buildSelectByPKSQL(tableName, pkName, len(pkValues))

	glog.Debugf(ctx, "[Seata] GenerateAfterImage SQL: %s, pkValues: %v", selectSQL, pkValues)

	// Execute query
	result, err := g.executeQuery(ctx, link, selectSQL, pkValues)
	if err != nil {
		return nil, gerror.Wrap(err, "failed to query after image")
	}

	// Convert to TableRecords
	return g.convertToTableRecords(ctx, tableName, result)
}

// GenerateInsertImage generates the after image for INSERT.
// Uses the last insert ID to query the inserted row.
func (g *ImageGenerator) GenerateInsertImage(
	ctx context.Context,
	link gdb.Link,
	tableName string,
	result sql.Result,
) (*TableRecords, error) {
	// Get last insert ID
	lastInsertID, err := result.LastInsertId()
	if err != nil {
		glog.Warningf(ctx, "[Seata] Failed to get LastInsertId: %v", err)
		// Some databases don't support LastInsertId, return empty
		return &TableRecords{
			TableName: tableName,
			Rows:      []Row{},
		}, nil
	}

	if lastInsertID == 0 {
		// No auto-increment primary key
		return &TableRecords{
			TableName: tableName,
			Rows:      []Row{},
		}, nil
	}

	// Query by primary key
	return g.GenerateAfterImage(ctx, link, tableName, []interface{}{lastInsertID})
}

// buildSelectSQL builds a SELECT SQL with FOR UPDATE.
func (g *ImageGenerator) buildSelectSQL(tableName string, condition string) string {
	if condition == "" {
		return fmt.Sprintf("SELECT * FROM %s FOR UPDATE", g.quoteTableName(tableName))
	}
	// condition 已经包含 WHERE 关键字（来自 GF 框架）
	// 例如: "WHERE `id`=?"
	return fmt.Sprintf("SELECT * FROM %s %s FOR UPDATE", g.quoteTableName(tableName), condition)
}

// buildSelectByPKSQL builds a SELECT SQL using primary key IN clause.
func (g *ImageGenerator) buildSelectByPKSQL(tableName string, pkName string, count int) string {
	placeholders := g.buildPlaceholders(count)
	return fmt.Sprintf("SELECT * FROM %s WHERE %s IN (%s)",
		g.quoteTableName(tableName),
		g.quoteColumnName(pkName),
		placeholders)
}

// buildPlaceholders builds placeholders for SQL IN clause.
func (g *ImageGenerator) buildPlaceholders(count int) string {
	if count == 0 {
		return ""
	}

	placeholders := "?"
	for i := 1; i < count; i++ {
		placeholders += ",?"
	}
	return placeholders
}

// executeQuery executes a query and returns the result.
func (g *ImageGenerator) executeQuery(
	ctx context.Context,
	link gdb.Link,
	sql string,
	args []interface{},
) (gdb.Result, error) {
	// Use Core's DoQuery which handles link correctly
	return g.resource.gfCore.DoQuery(ctx, link, sql, args...)
}

// convertToTableRecords converts gdb.Result to TableRecords.
func (g *ImageGenerator) convertToTableRecords(
	ctx context.Context,
	tableName string,
	result gdb.Result,
) (*TableRecords, error) {
	records := &TableRecords{
		TableName: tableName,
		Rows:      make([]Row, 0, len(result)),
	}

	if len(result) == 0 {
		return records, nil
	}

	// Get table metadata (field types and primary key)
	fieldTypes, err := g.getTableFieldTypes(ctx, tableName)
	if err != nil {
		return nil, err
	}

	pkName, err := g.getPrimaryKeyName(ctx, tableName)
	if err != nil {
		glog.Warningf(ctx, "[Seata] Failed to get primary key for table %s: %v", tableName, err)
		pkName = "" // Continue without primary key info
	}

	// Convert each record to Row
	for _, record := range result {
		row := Row{
			Fields: make([]Field, 0, len(record)),
		}

		for columnName, value := range record {
			field := Field{
				Name:  columnName,
				Type:  g.getSQLType(columnName, fieldTypes),
				Value: g.convertFieldValue(value),
			}

			// Check if it's a primary key
			if columnName == pkName {
				field.KeyType = KeyTypePrimaryKey
			} else {
				field.KeyType = KeyTypeCommon
			}

			row.Fields = append(row.Fields, field)
		}

		records.Rows = append(records.Rows, row)
	}

	return records, nil
}

// getTableFieldTypes gets the field types for a table.
func (g *ImageGenerator) getTableFieldTypes(ctx context.Context, tableName string) (map[string]string, error) {
	// Use gdb's TableFields to get field information
	fields, err := g.resource.gfCore.GetDB().TableFields(ctx, tableName)
	if err != nil {
		return nil, gerror.Wrapf(err, "failed to get table fields for %s", tableName)
	}

	fieldTypes := make(map[string]string)
	for name, field := range fields {
		fieldTypes[name] = field.Type
	}

	return fieldTypes, nil
}

// getPrimaryKeyName gets the primary key name for a table.
func (g *ImageGenerator) getPrimaryKeyName(ctx context.Context, tableName string) (string, error) {
	fields, err := g.resource.gfCore.GetDB().TableFields(ctx, tableName)
	if err != nil {
		return "", gerror.Wrapf(err, "failed to get table fields for %s", tableName)
	}

	for name, field := range fields {
		if field.Key == "PRI" { // MySQL primary key indicator
			return name, nil
		}
	}

	return "", gerror.Newf("no primary key found for table %s", tableName)
}

// getSQLType gets the SQL type code for a field.
// This is a simplified implementation, mapping to common SQL types.
func (g *ImageGenerator) getSQLType(columnName string, fieldTypes map[string]string) int32 {
	fieldType, ok := fieldTypes[columnName]
	if !ok {
		return 0 // Unknown type
	}

	// Map MySQL types to SQL standard type codes
	// Reference: java.sql.Types
	switch {
	case contains(fieldType, "int"):
		return 4 // INTEGER
	case contains(fieldType, "varchar"), contains(fieldType, "char"):
		return 12 // VARCHAR
	case contains(fieldType, "text"):
		return -1 // LONGVARCHAR
	case contains(fieldType, "decimal"), contains(fieldType, "numeric"):
		return 3 // DECIMAL
	case contains(fieldType, "float"):
		return 6 // FLOAT
	case contains(fieldType, "double"):
		return 8 // DOUBLE
	case contains(fieldType, "datetime"), contains(fieldType, "timestamp"):
		return 93 // TIMESTAMP
	case contains(fieldType, "date"):
		return 91 // DATE
	case contains(fieldType, "time"):
		return 92 // TIME
	case contains(fieldType, "blob"):
		return -4 // BLOB
	default:
		return 0 // OTHER
	}
}

// convertFieldValue converts a field value to a JSON-serializable format.
func (g *ImageGenerator) convertFieldValue(value interface{}) interface{} {
	if value == nil {
		return nil
	}

	// Convert []byte to string for better JSON serialization
	if bytes, ok := value.([]byte); ok {
		return string(bytes)
	}

	// Use gconv for other conversions
	return gconv.String(value)
}

// quoteTableName adds quotes to table name.
func (g *ImageGenerator) quoteTableName(tableName string) string {
	charLeft, charRight := g.resource.gfCore.GetChars()
	return charLeft + tableName + charRight
}

// quoteColumnName adds quotes to column name.
func (g *ImageGenerator) quoteColumnName(columnName string) string {
	charLeft, charRight := g.resource.gfCore.GetChars()
	return charLeft + columnName + charRight
}

// contains checks if a string contains a substring (case-insensitive).
func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// ExtractPrimaryKeyValues extracts primary key values from TableRecords.
func (records *TableRecords) ExtractPrimaryKeyValues() []interface{} {
	pkValues := make([]interface{}, 0)

	for _, row := range records.Rows {
		for _, field := range row.Fields {
			if field.KeyType == KeyTypePrimaryKey {
				pkValues = append(pkValues, field.Value)
				break // Only one primary key per row
			}
		}
	}

	return pkValues
}
