// Copyright GoFrame Author(https://goframe.org). All Rights Reserved.
//
// This Source Code Form is subject to the terms of the MIT License.
// If a copy of the MIT was not distributed with this file,
// You can obtain one at https://github.com/gogf/gf.

package seata_mysql

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/gogf/gf/v2/os/gcache"
	"github.com/gogf/gf/v2/os/gctx"
	"github.com/gogf/gf/v2/text/gregex"
	"github.com/gogf/gf/v2/text/gstr"
)

// 全局 SQL 解析器缓存
var (
	globalSQLParserCache *gcache.Cache
	globalCacheCtx       context.Context // 缓存操作用的全局上下文
	once                 sync.Once
)

// getGlobalSQLParserCache 获取全局 SQL 解析器缓存（单例）
func getGlobalSQLParserCache() *gcache.Cache {
	once.Do(func() {
		// 创建 LRU 缓存，容量 500
		globalSQLParserCache = gcache.New(500)
		// 初始化全局上下文（只初始化一次）
		globalCacheCtx = gctx.GetInitCtx()
	})
	return globalSQLParserCache
}

// SQLType SQL 类型
type SQLType int

const (
	SQLTypeUnknown SQLType = iota
	SQLTypeSelect
	SQLTypeInsert
	SQLTypeUpdate
	SQLTypeDelete
	SQLTypeDDL // CREATE, ALTER, DROP 等
)

// String 返回 SQL 类型的字符串表示
func (t SQLType) String() string {
	switch t {
	case SQLTypeSelect:
		return "SELECT"
	case SQLTypeInsert:
		return "INSERT"
	case SQLTypeUpdate:
		return "UPDATE"
	case SQLTypeDelete:
		return "DELETE"
	case SQLTypeDDL:
		return "DDL"
	default:
		return "UNKNOWN"
	}
}

// ParseSQLType 从字符串解析 SQL 类型
func ParseSQLType(s string) SQLType {
	switch s {
	case "SELECT":
		return SQLTypeSelect
	case "INSERT":
		return SQLTypeInsert
	case "UPDATE":
		return SQLTypeUpdate
	case "DELETE":
		return SQLTypeDelete
	case "DDL":
		return SQLTypeDDL
	default:
		return SQLTypeUnknown
	}
}

// SQLParser SQL 解析器
type SQLParser struct {
	sql string
}

// NewSQLParser 创建 SQL 解析器
func NewSQLParser(sql string) *SQLParser {
	return &SQLParser{
		sql: strings.TrimSpace(sql),
	}
}

// GetSQLType 获取 SQL 类型（带缓存）
func (p *SQLParser) GetSQLType() SQLType {
	// 尝试从缓存获取
	cache := getGlobalSQLParserCache()
	cacheKey := "sqltype:" + p.sql

	if value, err := cache.Get(globalCacheCtx, cacheKey); err == nil && value != nil {
		return ParseSQLType(value.String())
	}

	// 缓存未命中，执行解析
	upperSQL := gstr.ToUpper(p.sql)
	upperSQL = gstr.Trim(upperSQL)

	var sqlType SQLType
	if gstr.HasPrefix(upperSQL, "SELECT") || gstr.HasPrefix(upperSQL, "(SELECT") {
		sqlType = SQLTypeSelect
	} else if gstr.HasPrefix(upperSQL, "INSERT") {
		sqlType = SQLTypeInsert
	} else if gstr.HasPrefix(upperSQL, "UPDATE") {
		sqlType = SQLTypeUpdate
	} else if gstr.HasPrefix(upperSQL, "DELETE") {
		sqlType = SQLTypeDelete
	} else if gstr.HasPrefix(upperSQL, "CREATE") ||
		gstr.HasPrefix(upperSQL, "ALTER") ||
		gstr.HasPrefix(upperSQL, "DROP") ||
		gstr.HasPrefix(upperSQL, "TRUNCATE") {
		sqlType = SQLTypeDDL
	} else {
		sqlType = SQLTypeUnknown
	}

	// 缓存解析结果（10分钟过期）
	_ = cache.Set(globalCacheCtx, cacheKey, sqlType.String(), 10*time.Minute)

	return sqlType
}

// GetTableName 获取表名（带缓存）
func (p *SQLParser) GetTableName() string {
	// 尝试从缓存获取
	cache := getGlobalSQLParserCache()
	cacheKey := "table:" + p.sql

	if value, err := cache.Get(globalCacheCtx, cacheKey); err == nil && value != nil {
		return value.String()
	}

	// 缓存未命中，执行解析
	tableName := p.getTableNameWithoutCache()

	// 缓存解析结果
	_ = cache.Set(globalCacheCtx, cacheKey, tableName, 10*time.Minute)

	return tableName
}

// getTableNameWithoutCache 获取表名（不使用缓存）
func (p *SQLParser) getTableNameWithoutCache() string {
	sqlType := p.GetSQLType()

	switch sqlType {
	case SQLTypeInsert:
		return p.getInsertTableName()
	case SQLTypeUpdate:
		return p.getUpdateTableName()
	case SQLTypeDelete:
		return p.getDeleteTableName()
	case SQLTypeSelect:
		return p.getSelectTableName()
	default:
		return ""
	}
}

// getInsertTableName 从 INSERT 语句提取表名
// INSERT INTO users (name) VALUES ('test')
// INSERT INTO `users` (name) VALUES ('test')
func (p *SQLParser) getInsertTableName() string {
	// 匹配: INSERT INTO table_name
	pattern := `(?i)INSERT\s+INTO\s+` + "`?" + `([a-zA-Z0-9_]+)` + "`?"
	match, _ := gregex.MatchString(pattern, p.sql)
	if len(match) >= 2 {
		return strings.Trim(match[1], "`\"")
	}
	return ""
}

// getUpdateTableName 从 UPDATE 语句提取表名
// UPDATE users SET name='test' WHERE id=1
// UPDATE `users` SET name='test' WHERE id=1
func (p *SQLParser) getUpdateTableName() string {
	// 匹配: UPDATE table_name SET
	pattern := `(?i)UPDATE\s+` + "`?" + `([a-zA-Z0-9_]+)` + "`?" + `\s+SET`
	match, _ := gregex.MatchString(pattern, p.sql)
	if len(match) >= 2 {
		return strings.Trim(match[1], "`\"")
	}
	return ""
}

// getDeleteTableName 从 DELETE 语句提取表名
// DELETE FROM users WHERE id=1
// DELETE FROM `users` WHERE id=1
func (p *SQLParser) getDeleteTableName() string {
	// 匹配: DELETE FROM table_name
	pattern := `(?i)DELETE\s+FROM\s+` + "`?" + `([a-zA-Z0-9_]+)` + "`?"
	match, _ := gregex.MatchString(pattern, p.sql)
	if len(match) >= 2 {
		return strings.Trim(match[1], "`\"")
	}
	return ""
}

// getSelectTableName 从 SELECT 语句提取表名（简化版，只支持单表）
// SELECT * FROM users WHERE id=1
// SELECT * FROM `users` WHERE id=1
func (p *SQLParser) getSelectTableName() string {
	// 匹配: FROM table_name
	pattern := `(?i)FROM\s+` + "`?" + `([a-zA-Z0-9_]+)` + "`?" + `(\s+WHERE|\s+ORDER|\s+LIMIT|\s+GROUP|$)`
	match, _ := gregex.MatchString(pattern, p.sql)
	if len(match) >= 2 {
		return strings.Trim(match[1], "`\"")
	}
	return ""
}

// GetWhereClause 获取 WHERE 子句
func (p *SQLParser) GetWhereClause() string {
	// 匹配 WHERE 子句
	// WHERE id=1 AND name='test'
	// WHERE id=1 ORDER BY id
	// WHERE id=1 LIMIT 10
	pattern := `(?i)WHERE\s+(.+?)(\s+ORDER\s+BY|\s+LIMIT|\s+GROUP\s+BY|\s+FOR\s+UPDATE|$)`
	match, _ := gregex.MatchString(pattern, p.sql)
	if len(match) >= 2 {
		return strings.TrimSpace(match[1])
	}
	return ""
}

// HasWhereClause 检查是否有 WHERE 子句
func (p *SQLParser) HasWhereClause() bool {
	return gstr.ContainsI(p.sql, " WHERE ")
}

// GetSetClause 获取 UPDATE 的 SET 子句
func (p *SQLParser) GetSetClause() string {
	// 匹配 SET 子句
	// SET name='test', age=20 WHERE id=1
	pattern := `(?i)SET\s+(.+?)\s+WHERE`
	match, _ := gregex.MatchString(pattern, p.sql)
	if len(match) >= 2 {
		return strings.TrimSpace(match[1])
	}

	// 没有 WHERE 的情况
	pattern = `(?i)SET\s+(.+?)$`
	match, _ = gregex.MatchString(pattern, p.sql)
	if len(match) >= 2 {
		return strings.TrimSpace(match[1])
	}

	return ""
}

// IsMultiTable 检查是否是多表操作（阶段二不支持）
func (p *SQLParser) IsMultiTable() bool {
	tableName := p.GetTableName()
	if tableName == "" {
		return false
	}

	// 简单检查：是否包含 JOIN
	upperSQL := gstr.ToUpper(p.sql)
	return gstr.Contains(upperSQL, " JOIN ")
}

// BuildBeforeImageSQL 构建查询 beforeImage 的 SQL
// 用于 UPDATE 和 DELETE 操作
func (p *SQLParser) BuildBeforeImageSQL() string {
	sqlType := p.GetSQLType()
	tableName := p.GetTableName()

	if tableName == "" {
		return ""
	}

	switch sqlType {
	case SQLTypeUpdate, SQLTypeDelete:
		whereClause := p.GetWhereClause()
		if whereClause == "" {
			// 没有 WHERE 子句，查询全表（危险操作，应该限制）
			return "SELECT * FROM " + p.quoteTableName(tableName) + " FOR UPDATE"
		}
		// 有 WHERE 子句
		return "SELECT * FROM " + p.quoteTableName(tableName) +
			" WHERE " + whereClause + " FOR UPDATE"

	default:
		return ""
	}
}

// quoteTableName 给表名添加引号
func (p *SQLParser) quoteTableName(tableName string) string {
	// 如果已经有引号，直接返回
	if gstr.HasPrefix(tableName, "`") || gstr.HasPrefix(tableName, "\"") {
		return tableName
	}
	return "`" + tableName + "`"
}

// NeedUndoLog 判断是否需要生成 undo log
func (p *SQLParser) NeedUndoLog() bool {
	sqlType := p.GetSQLType()
	return sqlType == SQLTypeInsert ||
		sqlType == SQLTypeUpdate ||
		sqlType == SQLTypeDelete
}
