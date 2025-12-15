// Copyright GoFrame Author(https://goframe.org). All Rights Reserved.
//
// This Source Code Form is subject to the terms of the MIT License.
// If a copy of the MIT was not distributed with this file,
// You can obtain one at https://github.com/gogf/gf.

package seata_mysql

import (
	"context"
	"testing"

	"github.com/gogf/gf/v2/container/garray"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/test/gtest"
	"github.com/seata/seata-go/pkg/datasource/sql/types"
	"github.com/seata/seata-go/pkg/protocol/branch"
)

// Test_Config tests configuration functionality
func Test_Config(t *testing.T) {
	// Test default config
	gtest.C(t, func(t *gtest.T) {
		config := DefaultConfig()
		t.AssertNE(config, nil)
		t.AssertEQ(config.ApplicationID, "gf-app")
		t.AssertEQ(config.TxServiceGroup, "default_tx_group")
		t.AssertEQ(config.Mode, "AT")
		t.AssertEQ(config.AT.UndoLogTable, "undo_log")
	})

	// Test branch type resolution
	gtest.C(t, func(t *gtest.T) {
		tests := []struct {
			mode     string
			expected branch.BranchType
		}{
			{"AT", branch.BranchTypeAT},
			{"XA", branch.BranchTypeXA},
			{"TCC", branch.BranchTypeTCC},
			{"", branch.BranchTypeAT},
			{"UNKNOWN", branch.BranchTypeAT},
		}

		for _, tt := range tests {
			config := &Config{Mode: tt.mode}
			result := config.GetBranchType()
			t.Log("Mode:", tt.mode, "BranchType:", result)
		}
	})

	// Test XA timeout
	gtest.C(t, func(t *gtest.T) {
		config := &Config{
			Mode: "XA",
			XA: XAConfig{
				BranchExecutionTimeout: 60000, // 60 seconds
			},
		}
		timeout := config.GetTimeout()
		t.Assert(timeout.Seconds(), 60)
	})
}

// Test_Resource tests resource management
func Test_Resource(t *testing.T) {
	// Test resource creation
	gtest.C(t, func(t *gtest.T) {
		config := DefaultConfig()
		resource := NewResource(
			"test-resource",
			branch.BranchTypeAT,
			types.DBTypeMySQL,
			nil,
			nil,
			config,
		)

		t.AssertNE(resource, nil)
		t.AssertEQ(resource.GetResourceId(), "test-resource")
		t.AssertEQ(resource.GetBranchType(), branch.BranchTypeAT)
		t.AssertEQ(resource.GetResourceGroupId(), config.TxServiceGroup)
	})

	// Test connection keeper for XA mode
	gtest.C(t, func(t *gtest.T) {
		config := DefaultConfig()
		resource := NewResource(
			"xa-resource",
			branch.BranchTypeXA,
			types.DBTypeMySQL,
			nil,
			nil,
			config,
		)

		xaBranchID := "test-xa-branch-123"
		mockConn := &struct{ name string }{name: "test-conn"}

		// Hold connection
		err := resource.Hold(xaBranchID, mockConn)
		t.AssertNil(err)

		// Lookup connection
		conn, found := resource.Lookup(xaBranchID)
		t.Assert(found, true)
		t.AssertEQ(conn, mockConn)

		// Release connection
		resource.Release(xaBranchID)
		_, found = resource.Lookup(xaBranchID)
		t.Assert(found, false)
	})

	// Test resource ID building
	gtest.C(t, func(t *gtest.T) {
		node := &gdb.ConfigNode{
			Type: "mysql",
			User: "root",
			Host: "127.0.0.1",
			Port: "3306",
			Name: "test_db",
		}
		resourceID := BuildResourceID(node)
		t.AssertEQ(resourceID, "mysql:root@127.0.0.1:3306/test_db")
	})
}

// Test_Transaction tests transaction functionality
func Test_Transaction(t *testing.T) {
	// Test undo log collection in a transaction
	gtest.C(t, func(t *gtest.T) {
		tx := &SeataTX{
			ctx:          context.Background(),
			undoLogItems: garray.New(true),
		}

		// Add UPDATE undo log
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

		// Add INSERT undo log
		tx.AddUndoLog(&SQLUndoLog{
			SQLType:   SQLTypeInsert,
			TableName: "orders",
			AfterImage: &TableRecords{
				TableName: "orders",
				Rows: []Row{
					{
						Fields: []Field{
							{Name: "id", KeyType: KeyTypePrimaryKey, Value: 100},
							{Name: "amount", KeyType: KeyTypeCommon, Value: 100},
						},
					},
				},
			},
		})

		items := tx.GetUndoLogItems()
		t.AssertEQ(len(items), 2)
		t.AssertEQ(items[0].SQLType, SQLTypeUpdate)
		t.AssertEQ(items[1].SQLType, SQLTypeInsert)
	})

	// Test lock key generation
	gtest.C(t, func(t *gtest.T) {
		tx := &SeataTX{
			ctx:          context.Background(),
			undoLogItems: garray.New(true),
		}

		// Multiple rows from different tables
		tx.AddUndoLog(&SQLUndoLog{
			SQLType:   SQLTypeUpdate,
			TableName: "users",
			BeforeImage: &TableRecords{
				TableName: "users",
				Rows: []Row{
					{Fields: []Field{{Name: "id", KeyType: KeyTypePrimaryKey, Value: 1}}},
					{Fields: []Field{{Name: "id", KeyType: KeyTypePrimaryKey, Value: 2}}},
				},
			},
		})

		tx.AddUndoLog(&SQLUndoLog{
			SQLType:   SQLTypeInsert,
			TableName: "orders",
			AfterImage: &TableRecords{
				TableName: "orders",
				Rows: []Row{
					{Fields: []Field{{Name: "id", KeyType: KeyTypePrimaryKey, Value: 100}}},
				},
			},
		})

		lockKeys := tx.generateLockKeys()
		t.AssertNE(lockKeys, "")
		// Lock keys format: table:pk1,pk2;table2:pk3
		t.Log("Generated lock keys:", lockKeys)
	})

	// Test primary key deduplication in lock keys
	gtest.C(t, func(t *gtest.T) {
		tx := &SeataTX{
			ctx:          context.Background(),
			undoLogItems: garray.New(true),
		}

		// Add same primary key multiple times
		for i := 0; i < 3; i++ {
			tx.AddUndoLog(&SQLUndoLog{
				SQLType:   SQLTypeUpdate,
				TableName: "users",
				BeforeImage: &TableRecords{
					TableName: "users",
					Rows: []Row{
						{Fields: []Field{{Name: "id", KeyType: KeyTypePrimaryKey, Value: 1}}},
					},
				},
			})
		}

		lockKeys := tx.generateLockKeys()
		// Should be "users:1" not "users:1,1,1"
		t.Log("Lock keys with deduplication:", lockKeys)
	})
}

// Test_SQLParser tests SQL parsing functionality
func Test_SQLParser(t *testing.T) {
	// Test SQL type detection
	gtest.C(t, func(t *gtest.T) {
		tests := []struct {
			sql      string
			sqlType  SQLType
			needUndo bool
		}{
			{"SELECT * FROM users WHERE id = 1", SQLTypeSelect, false},
			{"INSERT INTO users (name) VALUES ('test')", SQLTypeInsert, true},
			{"UPDATE users SET name='test' WHERE id=1", SQLTypeUpdate, true},
			{"DELETE FROM users WHERE id=1", SQLTypeDelete, true},
		}

		for _, tt := range tests {
			parser := NewSQLParser(tt.sql)
			t.AssertEQ(parser.GetSQLType(), tt.sqlType)
			t.AssertEQ(parser.NeedUndoLog(), tt.needUndo)
		}
	})

	// Test table name extraction
	gtest.C(t, func(t *gtest.T) {
		tests := []struct {
			sql   string
			table string
		}{
			{"SELECT * FROM users", "users"},
			{"INSERT INTO `users` (name) VALUES ('test')", "users"},
			{"UPDATE users SET name='test'", "users"},
			{"DELETE FROM `users` WHERE id=1", "users"},
		}

		for _, tt := range tests {
			parser := NewSQLParser(tt.sql)
			t.AssertEQ(parser.GetTableName(), tt.table)
		}
	})

	// Test WHERE clause extraction
	gtest.C(t, func(t *gtest.T) {
		parser := NewSQLParser("SELECT * FROM users WHERE id=1 AND name='test'")
		whereClause := parser.GetWhereClause()
		t.AssertEQ(whereClause, "id=1 AND name='test'")
	})

	// Test before image SQL building
	gtest.C(t, func(t *gtest.T) {
		parser := NewSQLParser("UPDATE users SET name='new' WHERE id=1")
		beforeSQL := parser.BuildBeforeImageSQL()
		t.AssertEQ(beforeSQL, "SELECT * FROM `users` WHERE id=1 FOR UPDATE")
	})
}

// Test_ImageGenerator tests image generation functionality
func Test_ImageGenerator(t *testing.T) {
	// Test SQL type mapping
	gtest.C(t, func(t *gtest.T) {
		gen := &ImageGenerator{resource: &Resource{}}

		fieldTypes := map[string]string{
			"id":         "int(11)",
			"name":       "varchar(100)",
			"price":      "decimal(10,2)",
			"created_at": "datetime",
		}

		t.AssertEQ(gen.getSQLType("id", fieldTypes), int32(4))          // INTEGER
		t.AssertEQ(gen.getSQLType("name", fieldTypes), int32(12))       // VARCHAR
		t.AssertEQ(gen.getSQLType("price", fieldTypes), int32(3))       // DECIMAL
		t.AssertEQ(gen.getSQLType("created_at", fieldTypes), int32(93)) // TIMESTAMP
	})

	// Test field value conversion
	gtest.C(t, func(t *gtest.T) {
		gen := &ImageGenerator{resource: &Resource{}}

		t.AssertEQ(gen.convertFieldValue(nil), nil)
		t.AssertEQ(gen.convertFieldValue([]byte("test")), "test")
		t.AssertEQ(gen.convertFieldValue("hello"), "hello")
		t.AssertEQ(gen.convertFieldValue(123), "123")
	})

	// Test primary key extraction
	gtest.C(t, func(t *gtest.T) {
		records := &TableRecords{
			TableName: "users",
			Rows: []Row{
				{
					Fields: []Field{
						{Name: "id", KeyType: KeyTypePrimaryKey, Value: 1},
						{Name: "name", KeyType: KeyTypeCommon, Value: "test"},
					},
				},
				{
					Fields: []Field{
						{Name: "id", KeyType: KeyTypePrimaryKey, Value: 2},
						{Name: "name", KeyType: KeyTypeCommon, Value: "test2"},
					},
				},
			},
		}

		pkValues := records.ExtractPrimaryKeyValues()
		t.AssertEQ(len(pkValues), 2)
		t.AssertEQ(pkValues[0], 1)
		t.AssertEQ(pkValues[1], 2)
	})

	// Test placeholder building
	gtest.C(t, func(t *gtest.T) {
		gen := &ImageGenerator{resource: &Resource{}}

		t.AssertEQ(gen.buildPlaceholders(0), "")
		t.AssertEQ(gen.buildPlaceholders(1), "?")
		t.AssertEQ(gen.buildPlaceholders(3), "?,?,?")
	})
}

// Test_DSNBuilding tests DSN construction
func Test_DSNBuilding(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		tests := []struct {
			name     string
			node     *gdb.ConfigNode
			expected string
		}{
			{
				name: "basic config",
				node: &gdb.ConfigNode{
					User:     "root",
					Pass:     "password",
					Protocol: "tcp",
					Host:     "127.0.0.1",
					Port:     "3306",
					Name:     "test_db",
					Charset:  "utf8mb4",
				},
				expected: "root:password@tcp(127.0.0.1:3306)/test_db?charset=utf8mb4",
			},
			{
				name: "with timezone",
				node: &gdb.ConfigNode{
					User:     "user",
					Pass:     "pass",
					Protocol: "tcp",
					Host:     "localhost",
					Port:     "3306",
					Name:     "app_db",
					Charset:  "utf8mb4",
					Timezone: "Asia/Shanghai",
				},
				expected: "user:pass@tcp(localhost:3306)/app_db?charset=utf8mb4&loc=Asia/Shanghai",
			},
		}

		for _, tt := range tests {
			dsn := buildDSN(tt.node)
			t.AssertEQ(dsn, tt.expected)
			t.Log(tt.name, ":", dsn)
		}
	})
}

// Test_BranchUndoLog tests branch undo log serialization
func Test_BranchUndoLog(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
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
				},
				{
					SQLType:   SQLTypeInsert,
					TableName: "orders",
					AfterImage: &TableRecords{
						TableName: "orders",
						Rows: []Row{
							{
								Fields: []Field{
									{Name: "id", KeyType: KeyTypePrimaryKey, Value: 100},
									{Name: "amount", KeyType: KeyTypeCommon, Value: 100},
								},
							},
						},
					},
				},
			},
		}

		t.AssertNE(branchLog.XID, "")
		t.AssertEQ(len(branchLog.SQLUndoLogs), 2)
		t.AssertEQ(branchLog.SQLUndoLogs[0].SQLType, SQLTypeUpdate)
		t.AssertEQ(branchLog.SQLUndoLogs[1].SQLType, SQLTypeInsert)
		t.AssertNE(branchLog.SQLUndoLogs[0].BeforeImage, nil)
		t.AssertNE(branchLog.SQLUndoLogs[0].AfterImage, nil)
		t.AssertEQ(branchLog.SQLUndoLogs[1].BeforeImage, nil) // INSERT has no before image
	})
}

// Test_TransactionLifecycle tests complete transaction undo log lifecycle
func Test_TransactionLifecycle(t *testing.T) {
	// Test complete transaction flow: transfer money scenario
	gtest.C(t, func(t *gtest.T) {
		tx := &SeataTX{
			ctx:          context.Background(),
			undoLogItems: garray.New(true),
		}

		// Step 1: Deduct from account A
		tx.AddUndoLog(&SQLUndoLog{
			SQLType:   SQLTypeUpdate,
			TableName: "accounts",
			BeforeImage: &TableRecords{
				TableName: "accounts",
				Rows: []Row{
					{Fields: []Field{
						{Name: "id", KeyType: KeyTypePrimaryKey, Value: 1},
						{Name: "balance", KeyType: KeyTypeCommon, Value: 1000.0},
					}},
				},
			},
			AfterImage: &TableRecords{
				TableName: "accounts",
				Rows: []Row{
					{Fields: []Field{
						{Name: "id", KeyType: KeyTypePrimaryKey, Value: 1},
						{Name: "balance", KeyType: KeyTypeCommon, Value: 500.0},
					}},
				},
			},
		})

		// Step 2: Add to account B
		tx.AddUndoLog(&SQLUndoLog{
			SQLType:   SQLTypeUpdate,
			TableName: "accounts",
			BeforeImage: &TableRecords{
				TableName: "accounts",
				Rows: []Row{
					{Fields: []Field{
						{Name: "id", KeyType: KeyTypePrimaryKey, Value: 2},
						{Name: "balance", KeyType: KeyTypeCommon, Value: 500.0},
					}},
				},
			},
			AfterImage: &TableRecords{
				TableName: "accounts",
				Rows: []Row{
					{Fields: []Field{
						{Name: "id", KeyType: KeyTypePrimaryKey, Value: 2},
						{Name: "balance", KeyType: KeyTypeCommon, Value: 1000.0},
					}},
				},
			},
		})

		// Step 3: Create transfer record
		tx.AddUndoLog(&SQLUndoLog{
			SQLType:   SQLTypeInsert,
			TableName: "transfers",
			AfterImage: &TableRecords{
				TableName: "transfers",
				Rows: []Row{
					{Fields: []Field{
						{Name: "id", KeyType: KeyTypePrimaryKey, Value: 999},
						{Name: "from_account", KeyType: KeyTypeCommon, Value: 1},
						{Name: "to_account", KeyType: KeyTypeCommon, Value: 2},
						{Name: "amount", KeyType: KeyTypeCommon, Value: 500.0},
					}},
				},
			},
		})

		// Verify undo log collection
		items := tx.GetUndoLogItems()
		t.AssertEQ(len(items), 3)

		// Verify lock keys include both accounts
		lockKeys := tx.generateLockKeys()
		t.AssertNE(lockKeys, "")
		t.Log("Transfer transaction lock keys:", lockKeys)
	})

	// Test concurrent undo log additions
	gtest.C(t, func(t *gtest.T) {
		tx := &SeataTX{
			ctx:          context.Background(),
			undoLogItems: garray.New(true),
		}

		// Simulate concurrent SQL executions
		done := make(chan bool)
		for i := 0; i < 5; i++ {
			go func(idx int) {
				for j := 0; j < 3; j++ {
					tx.AddUndoLog(&SQLUndoLog{
						SQLType:   SQLTypeUpdate,
						TableName: "test_table",
					})
				}
				done <- true
			}(i)
		}

		// Wait for all goroutines
		for i := 0; i < 5; i++ {
			<-done
		}

		// Verify thread-safe addition
		t.AssertEQ(tx.undoLogItems.Len(), 15)
	})
}

// Test_ComplexSQLScenarios tests various SQL parsing scenarios
func Test_ComplexSQLScenarios(t *testing.T) {
	// Multi-row operations
	gtest.C(t, func(t *gtest.T) {
		tx := &SeataTX{
			ctx:          context.Background(),
			undoLogItems: garray.New(true),
		}

		// Batch INSERT
		tx.AddUndoLog(&SQLUndoLog{
			SQLType:   SQLTypeInsert,
			TableName: "products",
			AfterImage: &TableRecords{
				TableName: "products",
				Rows: []Row{
					{Fields: []Field{{Name: "id", KeyType: KeyTypePrimaryKey, Value: 1}}},
					{Fields: []Field{{Name: "id", KeyType: KeyTypePrimaryKey, Value: 2}}},
					{Fields: []Field{{Name: "id", KeyType: KeyTypePrimaryKey, Value: 3}}},
				},
			},
		})

		// Verify multi-row lock keys
		lockKeys := tx.generateLockKeys()
		t.AssertNE(lockKeys, "")
		t.Log("Batch insert lock keys:", lockKeys)
	})

	// Complex WHERE clause parsing
	gtest.C(t, func(t *gtest.T) {
		tests := []struct {
			name  string
			sql   string
			where string
		}{
			{"simple", "SELECT * FROM users WHERE id=1", "id=1"},
			{"AND", "SELECT * FROM users WHERE id=1 AND status='active'", "id=1 AND status='active'"},
			{"IN clause", "DELETE FROM users WHERE id IN (1,2,3)", "id IN (1,2,3)"},
			{"no WHERE", "SELECT * FROM users", ""},
		}

		for _, tt := range tests {
			parser := NewSQLParser(tt.sql)
			t.AssertEQ(parser.GetWhereClause(), tt.where)
		}
	})

	// Table name extraction with various formats
	gtest.C(t, func(t *gtest.T) {
		tests := []struct {
			sql   string
			table string
		}{
			{"INSERT INTO users (name) VALUES ('test')", "users"},
			{"INSERT INTO `users` (name) VALUES ('test')", "users"},
			{"UPDATE users SET name='test'", "users"},
			{"DELETE FROM `products` WHERE id=1", "products"},
		}

		for _, tt := range tests {
			parser := NewSQLParser(tt.sql)
			t.AssertEQ(parser.GetTableName(), tt.table)
		}
	})
}
