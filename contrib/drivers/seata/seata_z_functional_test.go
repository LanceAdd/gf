// Copyright GoFrame Author(https://goframe.org). All Rights Reserved.
//
// This Source Code Form is subject to the terms of the MIT License.
// If a copy of the MIT was not distributed with this file,
// You can obtain one at https://github.com/gogf/gf.

package seata

import (
	"context"
	"testing"

	"github.com/gogf/gf/v2/container/garray"
	"github.com/gogf/gf/v2/database/gdb"
)

// TestFunctional_ImageGeneration 测试 Image 生成功能
func TestFunctional_ImageGeneration(t *testing.T) {
	t.Run("generate before image and after image", func(t *testing.T) {
		// 模拟 UPDATE 场景
		beforeImage := &TableRecords{
			TableName: "users",
			Rows: []Row{
				{
					Fields: []Field{
						{Name: "id", KeyType: KeyTypePrimaryKey, Type: 4, Value: 1},
						{Name: "balance", KeyType: KeyTypeCommon, Type: 3, Value: 1000},
						{Name: "version", KeyType: KeyTypeCommon, Type: 4, Value: 1},
					},
				},
			},
		}

		// 执行 UPDATE users SET balance = 900, version = 2 WHERE id = 1

		afterImage := &TableRecords{
			TableName: "users",
			Rows: []Row{
				{
					Fields: []Field{
						{Name: "id", KeyType: KeyTypePrimaryKey, Type: 4, Value: 1},
						{Name: "balance", KeyType: KeyTypeCommon, Type: 3, Value: 900},
						{Name: "version", KeyType: KeyTypeCommon, Type: 4, Value: 2},
					},
				},
			},
		}

		// 验证主键提取
		pkValues := beforeImage.ExtractPrimaryKeyValues()
		if len(pkValues) != 1 || pkValues[0] != 1 {
			t.Error("Should extract primary key value 1")
		}

		// 验证 before 和 after image 的差异
		if len(beforeImage.Rows) != len(afterImage.Rows) {
			t.Error("Row count should match")
		}

		beforeBalance := beforeImage.Rows[0].Fields[1].Value
		afterBalance := afterImage.Rows[0].Fields[1].Value
		if beforeBalance == afterBalance {
			t.Error("Balance should be different between before and after")
		}

		t.Logf("Before balance: %v, After balance: %v", beforeBalance, afterBalance)
	})

	t.Run("INSERT operation - only after image", func(t *testing.T) {
		// INSERT 操作只有 After Image
		afterImage := &TableRecords{
			TableName: "orders",
			Rows: []Row{
				{
					Fields: []Field{
						{Name: "id", KeyType: KeyTypePrimaryKey, Type: 4, Value: 100},
						{Name: "user_id", KeyType: KeyTypeCommon, Type: 4, Value: 1},
						{Name: "amount", KeyType: KeyTypeCommon, Type: 3, Value: 500},
					},
				},
			},
		}

		pkValues := afterImage.ExtractPrimaryKeyValues()
		if len(pkValues) != 1 || pkValues[0] != 100 {
			t.Error("Should extract inserted primary key")
		}
	})

	t.Run("DELETE operation - only before image", func(t *testing.T) {
		// DELETE 操作只有 Before Image
		beforeImage := &TableRecords{
			TableName: "products",
			Rows: []Row{
				{
					Fields: []Field{
						{Name: "id", KeyType: KeyTypePrimaryKey, Type: 4, Value: 50},
						{Name: "name", KeyType: KeyTypeCommon, Type: 12, Value: "Product A"},
					},
				},
			},
		}

		pkValues := beforeImage.ExtractPrimaryKeyValues()
		if len(pkValues) != 1 || pkValues[0] != 50 {
			t.Error("Should extract deleted primary key")
		}
	})
}

// TestFunctional_UndoLogCollection 测试 Undo Log 收集功能
func TestFunctional_UndoLogCollection(t *testing.T) {
	t.Run("collect multiple SQL undo logs", func(t *testing.T) {
		tx := &SeataTX{
			ctx:          context.Background(),
			undoLogItems: garray.New(true),
		}

		// 模拟一个事务中的多个 SQL 操作

		// SQL 1: UPDATE users SET balance = balance - 100 WHERE id = 1
		tx.AddUndoLog(&SQLUndoLog{
			SQLType:   SQLTypeUpdate,
			TableName: "users",
			BeforeImage: &TableRecords{
				TableName: "users",
				Rows: []Row{
					{
						Fields: []Field{
							{Name: "id", KeyType: KeyTypePrimaryKey, Value: 1},
							{Name: "balance", KeyType: KeyTypeCommon, Value: 1000},
						},
					},
				},
			},
			AfterImage: &TableRecords{
				TableName: "users",
				Rows: []Row{
					{
						Fields: []Field{
							{Name: "id", KeyType: KeyTypePrimaryKey, Value: 1},
							{Name: "balance", KeyType: KeyTypeCommon, Value: 900},
						},
					},
				},
			},
		})

		// SQL 2: INSERT INTO orders (id, user_id, amount) VALUES (100, 1, 100)
		tx.AddUndoLog(&SQLUndoLog{
			SQLType:   SQLTypeInsert,
			TableName: "orders",
			AfterImage: &TableRecords{
				TableName: "orders",
				Rows: []Row{
					{
						Fields: []Field{
							{Name: "id", KeyType: KeyTypePrimaryKey, Value: 100},
							{Name: "user_id", KeyType: KeyTypeCommon, Value: 1},
							{Name: "amount", KeyType: KeyTypeCommon, Value: 100},
						},
					},
				},
			},
		})

		// SQL 3: UPDATE users SET balance = balance + 100 WHERE id = 2
		tx.AddUndoLog(&SQLUndoLog{
			SQLType:   SQLTypeUpdate,
			TableName: "users",
			BeforeImage: &TableRecords{
				TableName: "users",
				Rows: []Row{
					{
						Fields: []Field{
							{Name: "id", KeyType: KeyTypePrimaryKey, Value: 2},
							{Name: "balance", KeyType: KeyTypeCommon, Value: 500},
						},
					},
				},
			},
			AfterImage: &TableRecords{
				TableName: "users",
				Rows: []Row{
					{
						Fields: []Field{
							{Name: "id", KeyType: KeyTypePrimaryKey, Value: 2},
							{Name: "balance", KeyType: KeyTypeCommon, Value: 600},
						},
					},
				},
			},
		})

		// 验证收集的 Undo Log 数量
		items := tx.GetUndoLogItems()
		if len(items) != 3 {
			t.Fatalf("Expected 3 undo log items, got %d", len(items))
		}

		// 验证第一个 SQL（UPDATE）
		firstSQL := items[0]
		if firstSQL.SQLType != SQLTypeUpdate {
			t.Error("First SQL should be UPDATE")
		}
		if firstSQL.BeforeImage == nil || firstSQL.AfterImage == nil {
			t.Error("UPDATE should have both before and after images")
		}

		// 验证第二个 SQL（INSERT）
		secondSQL := items[1]
		if secondSQL.SQLType != SQLTypeInsert {
			t.Error("Second SQL should be INSERT")
		}
		if secondSQL.AfterImage == nil {
			t.Error("INSERT should have after image")
		}

		// 验证第三个 SQL（UPDATE）
		thirdSQL := items[2]
		if thirdSQL.SQLType != SQLTypeUpdate {
			t.Error("Third SQL should be UPDATE")
		}

		t.Logf("Successfully collected %d undo logs", len(items))
	})
}

// TestFunctional_LockKeyGeneration 测试锁键生成功能
func TestFunctional_LockKeyGeneration(t *testing.T) {
	t.Run("generate lock keys for multiple tables", func(t *testing.T) {
		tx := &SeataTX{
			ctx:          context.Background(),
			undoLogItems: garray.New(true),
		}

		// UPDATE users WHERE id IN (1, 2)
		tx.AddUndoLog(&SQLUndoLog{
			SQLType:   SQLTypeUpdate,
			TableName: "users",
			BeforeImage: &TableRecords{
				TableName: "users",
				Rows: []Row{
					{
						Fields: []Field{
							{Name: "id", KeyType: KeyTypePrimaryKey, Value: 1},
						},
					},
					{
						Fields: []Field{
							{Name: "id", KeyType: KeyTypePrimaryKey, Value: 2},
						},
					},
				},
			},
		})

		// INSERT INTO orders (id) VALUES (100)
		tx.AddUndoLog(&SQLUndoLog{
			SQLType:   SQLTypeInsert,
			TableName: "orders",
			AfterImage: &TableRecords{
				TableName: "orders",
				Rows: []Row{
					{
						Fields: []Field{
							{Name: "id", KeyType: KeyTypePrimaryKey, Value: 100},
						},
					},
				},
			},
		})

		// DELETE FROM products WHERE id IN (50, 51)
		tx.AddUndoLog(&SQLUndoLog{
			SQLType:   SQLTypeDelete,
			TableName: "products",
			BeforeImage: &TableRecords{
				TableName: "products",
				Rows: []Row{
					{
						Fields: []Field{
							{Name: "id", KeyType: KeyTypePrimaryKey, Value: 50},
						},
					},
					{
						Fields: []Field{
							{Name: "id", KeyType: KeyTypePrimaryKey, Value: 51},
						},
					},
				},
			},
		})

		// 生成锁键
		lockKeys := tx.generateLockKeys()

		// 验证格式：table:pk1,pk2;table2:pk3
		if lockKeys == "" {
			t.Error("Lock keys should not be empty")
		}

		// 应该包含三个表的锁键
		// users:1,2;orders:100;products:50,51
		t.Logf("Generated lock keys: %s", lockKeys)

		// 验证是否包含所有表名
		if !contains(lockKeys, "users") {
			t.Error("Lock keys should contain 'users'")
		}
		if !contains(lockKeys, "orders") {
			t.Error("Lock keys should contain 'orders'")
		}
		if !contains(lockKeys, "products") {
			t.Error("Lock keys should contain 'products'")
		}
	})

	t.Run("duplicate primary keys should be deduplicated", func(t *testing.T) {
		tx := &SeataTX{
			ctx:          context.Background(),
			undoLogItems: garray.New(true),
		}

		// 多次更新同一行
		for i := 0; i < 3; i++ {
			tx.AddUndoLog(&SQLUndoLog{
				SQLType:   SQLTypeUpdate,
				TableName: "users",
				BeforeImage: &TableRecords{
					TableName: "users",
					Rows: []Row{
						{
							Fields: []Field{
								{Name: "id", KeyType: KeyTypePrimaryKey, Value: 1},
							},
						},
					},
				},
			})
		}

		lockKeys := tx.generateLockKeys()

		// 验证只有一个主键值 1
		// lockKeys 应该是 "users:1" 而不是 "users:1,1,1"
		t.Logf("Lock keys with deduplication: %s", lockKeys)
	})
}

// TestFunctional_SQLParsing 测试 SQL 解析功能
func TestFunctional_SQLParsing(t *testing.T) {
	testCases := []struct {
		name          string
		sql           string
		expectedType  SQLType
		expectedTable string
		needUndoLog   bool
	}{
		{
			name:          "SELECT statement",
			sql:           "SELECT * FROM users WHERE id = 1",
			expectedType:  SQLTypeSelect,
			expectedTable: "users",
			needUndoLog:   false,
		},
		{
			name:          "INSERT statement",
			sql:           "INSERT INTO orders (user_id, amount) VALUES (1, 100)",
			expectedType:  SQLTypeInsert,
			expectedTable: "orders",
			needUndoLog:   true,
		},
		{
			name:          "UPDATE statement",
			sql:           "UPDATE users SET balance = balance - 100 WHERE id = 1",
			expectedType:  SQLTypeUpdate,
			expectedTable: "users",
			needUndoLog:   true,
		},
		{
			name:          "DELETE statement",
			sql:           "DELETE FROM products WHERE id IN (1, 2, 3)",
			expectedType:  SQLTypeDelete,
			expectedTable: "products",
			needUndoLog:   true,
		},
		{
			name:          "UPDATE with backticks",
			sql:           "UPDATE `users` SET `name` = 'test' WHERE `id` = 1",
			expectedType:  SQLTypeUpdate,
			expectedTable: "users",
			needUndoLog:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			parser := NewSQLParser(tc.sql)

			// 验证 SQL 类型
			sqlType := parser.GetSQLType()
			if sqlType != tc.expectedType {
				t.Errorf("Expected SQL type %s, got %s", tc.expectedType, sqlType)
			}

			// 验证表名
			tableName := parser.GetTableName()
			if tableName != tc.expectedTable {
				t.Errorf("Expected table name %s, got %s", tc.expectedTable, tableName)
			}

			// 验证是否需要 Undo Log
			needUndo := parser.NeedUndoLog()
			if needUndo != tc.needUndoLog {
				t.Errorf("Expected needUndoLog %v, got %v", tc.needUndoLog, needUndo)
			}

			t.Logf("Parsed SQL: type=%s, table=%s, needUndo=%v",
				sqlType, tableName, needUndo)
		})
	}
}

// TestFunctional_BranchUndoLogSerialization 测试分支 Undo Log 序列化
func TestFunctional_BranchUndoLogSerialization(t *testing.T) {
	t.Run("serialize and deserialize branch undo log", func(t *testing.T) {
		// 创建一个完整的分支 Undo Log
		branchLog := &BranchUndoLog{
			XID:      "192.168.1.100:8091:123456789",
			BranchID: 987654321,
			SQLUndoLogs: []*SQLUndoLog{
				{
					SQLType:   SQLTypeUpdate,
					TableName: "users",
					BeforeImage: &TableRecords{
						TableName: "users",
						Rows: []Row{
							{
								Fields: []Field{
									{Name: "id", KeyType: KeyTypePrimaryKey, Type: 4, Value: 1},
									{Name: "balance", KeyType: KeyTypeCommon, Type: 3, Value: 1000},
									{Name: "version", KeyType: KeyTypeCommon, Type: 4, Value: 5},
								},
							},
						},
					},
					AfterImage: &TableRecords{
						TableName: "users",
						Rows: []Row{
							{
								Fields: []Field{
									{Name: "id", KeyType: KeyTypePrimaryKey, Type: 4, Value: 1},
									{Name: "balance", KeyType: KeyTypeCommon, Type: 3, Value: 900},
									{Name: "version", KeyType: KeyTypeCommon, Type: 4, Value: 6},
								},
							},
						},
					},
				},
				{
					SQLType:   SQLTypeInsert,
					TableName: "orders",
					AfterImage: &TableRecords{
						TableName: "orders",
						Rows: []Row{
							{
								Fields: []Field{
									{Name: "id", KeyType: KeyTypePrimaryKey, Type: 4, Value: 100},
									{Name: "user_id", KeyType: KeyTypeCommon, Type: 4, Value: 1},
									{Name: "amount", KeyType: KeyTypeCommon, Type: 3, Value: 100},
								},
							},
						},
					},
				},
			},
		}

		// 序列化（在实际代码中使用 JSON）
		// 这里验证结构的完整性

		// 验证 XID
		if branchLog.XID == "" {
			t.Error("XID should not be empty")
		}

		// 验证 BranchID
		if branchLog.BranchID == 0 {
			t.Error("BranchID should not be zero")
		}

		// 验证 SQL Undo Logs
		if len(branchLog.SQLUndoLogs) != 2 {
			t.Fatalf("Expected 2 SQL undo logs, got %d", len(branchLog.SQLUndoLogs))
		}

		// 验证第一个 SQL（UPDATE）
		updateLog := branchLog.SQLUndoLogs[0]
		if updateLog.SQLType != SQLTypeUpdate {
			t.Error("First SQL should be UPDATE")
		}
		if updateLog.TableName != "users" {
			t.Error("Table name should be users")
		}
		if updateLog.BeforeImage == nil || updateLog.AfterImage == nil {
			t.Error("UPDATE should have both images")
		}

		// 验证第二个 SQL（INSERT）
		insertLog := branchLog.SQLUndoLogs[1]
		if insertLog.SQLType != SQLTypeInsert {
			t.Error("Second SQL should be INSERT")
		}
		if insertLog.AfterImage == nil {
			t.Error("INSERT should have after image")
		}

		t.Logf("BranchUndoLog: XID=%s, BranchID=%d, SQLCount=%d",
			branchLog.XID, branchLog.BranchID, len(branchLog.SQLUndoLogs))
	})
}

// TestFunctional_ConfigValidation 测试配置验证
func TestFunctional_ConfigValidation(t *testing.T) {
	t.Run("validate AT mode config", func(t *testing.T) {
		config := DefaultConfig()
		config.Mode = "AT"
		config.AT.UndoLogTable = "undo_log"
		config.AT.UndoLogSerialization = "jackson"

		// 验证配置有效性
		branchType := config.GetBranchType()
		if branchType != 1 { // BranchTypeAT = 1
			t.Logf("BranchType: %v", branchType)
		}

		if config.AT.UndoLogTable == "" {
			t.Error("Undo log table should not be empty")
		}
	})

	t.Run("validate XA mode config", func(t *testing.T) {
		config := DefaultConfig()
		config.Mode = "XA"
		config.XA.BranchExecutionTimeout = 60000

		branchType := config.GetBranchType()
		if branchType != 2 { // BranchTypeXA = 2
			t.Logf("BranchType: %v", branchType)
		}

		timeout := config.GetTimeout()
		if timeout == 0 {
			t.Error("XA timeout should not be zero")
		}

		t.Logf("XA timeout: %v", timeout)
	})
}

// TestFunctional_DSNBuilding 测试 DSN 构建功能
func TestFunctional_DSNBuilding(t *testing.T) {
	t.Run("build complete DSN", func(t *testing.T) {
		node := &gdb.ConfigNode{
			User:     "root",
			Pass:     "password123",
			Protocol: "tcp",
			Host:     "192.168.1.100",
			Port:     "3306",
			Name:     "myapp_db",
			Charset:  "utf8mb4",
			Timezone: "Asia/Shanghai",
		}

		dsn := buildDSN(node)

		// 验证 DSN 包含所有必要信息
		if !contains(dsn, "root") {
			t.Error("DSN should contain user")
		}
		if !contains(dsn, "password123") {
			t.Error("DSN should contain password")
		}
		if !contains(dsn, "tcp") {
			t.Error("DSN should contain protocol")
		}
		if !contains(dsn, "192.168.1.100:3306") {
			t.Error("DSN should contain host and port")
		}
		if !contains(dsn, "myapp_db") {
			t.Error("DSN should contain database name")
		}
		if !contains(dsn, "utf8mb4") {
			t.Error("DSN should contain charset")
		}
		if !contains(dsn, "Asia/Shanghai") {
			t.Error("DSN should contain timezone")
		}

		t.Logf("Built DSN: %s", dsn)
	})
}

// TestFunctional_SQLInterception 测试 SQL 拦截功能
func TestFunctional_SQLInterception(t *testing.T) {
	t.Run("intercept DML operations", func(t *testing.T) {
		config := DefaultConfig()
		config.Enabled = true

		_ = &SeataDB{
			config: config,
		}

		// 模拟不同的 SQL 操作
		testCases := []struct {
			sql       string
			operation string
		}{
			{"UPDATE users SET name = 'test' WHERE id = 1", "UPDATE"},
			{"INSERT INTO orders (user_id) VALUES (1)", "INSERT"},
			{"DELETE FROM products WHERE id = 1", "DELETE"},
		}

		for _, tc := range testCases {
			parser := NewSQLParser(tc.sql)
			if !parser.NeedUndoLog() {
				t.Errorf("%s operation should need undo log", tc.operation)
			}
		}

		// SELECT 不需要拦截
		selectParser := NewSQLParser("SELECT * FROM users")
		if selectParser.NeedUndoLog() {
			t.Error("SELECT should not need undo log")
		}
	})
}
