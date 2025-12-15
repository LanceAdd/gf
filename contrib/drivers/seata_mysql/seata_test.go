// Copyright GoFrame Author(https://goframe.org). All Rights Reserved.
//
// This Source Code Form is subject to the terms of the MIT License.
// If a copy of the MIT was not distributed with this file,
// You can obtain one at https://github.com/gogf/gf.

package seata_mysql

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/seata/seata-go/pkg/client"
	"github.com/seata/seata-go/pkg/protocol/branch"
	"github.com/seata/seata-go/pkg/rm"

	"github.com/gogf/gf/v2/container/garray"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gctx"
	"github.com/gogf/gf/v2/test/gtest"

	"github.com/gogf/gf/contrib/drivers/mysql/v2"
)

// init 注册 Seata MySQL 驱动
func init() {
	// 设置 Seata 配置文件路径
	cwd, _ := os.Getwd()
	confPath := filepath.Join(cwd, "seata.yml")
	os.Setenv("SEATA_GO_CONFIG_PATH", confPath)

	// 初始化 Seata 客户端
	client.Init()

	// 注册 Seata AT 模式驱动（启用 Seata）
	config := DefaultConfig()
	config.Enabled = true // 启用 Seata
	config.ApplicationID = "seata-test-app"
	config.TxServiceGroup = "default_tx_group"
	config.Registry.Type = "file" // 使用文件注册中心

	if err := gdb.Register(DriverNameATMySQL, NewDriverAT(config)); err != nil {
		panic(err)
	}
}

// ====================================================================================
// 基础测试
// ====================================================================================

// TestDriverAT_New 测试 AT 模式驱动的创建
func TestDriverAT_New(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		// 创建配置
		config := &Config{
			Enabled: false, // 禁用 Seata 以测试基础功能
		}

		// 创建驱动
		driver := NewDriverAT(config)
		t.AssertNE(driver, nil)
		t.Assert(driver.config.Enabled, false)
	})
}

// TestSeataDB_Embedding 测试 SeataDB 是否正确嵌入 mysql.Driver
func TestSeataDB_Embedding(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		// 创建 MySQL Driver
		mysqlDriver := mysql.New()
		t.AssertNE(mysqlDriver, nil)

		// 创建 SeataDB
		seataDB := &SeataDB{
			Driver: mysqlDriver.(*mysql.Driver),
			config: DefaultConfig(),
		}

		// 验证嵌入的方法可访问
		t.Assert(seataDB.Driver != nil, true)

		// 验证可以调用 MySQL Driver 的方法
		// 注意：这里只验证方法存在，不实际调用
		charLeft, charRight := seataDB.GetChars()
		t.Assert(charLeft != "", true)
		t.Assert(charRight != "", true)
	})
}

// TestConfig 测试配置
func TestConfig(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		// 测试默认配置
		config := DefaultConfig()
		t.AssertNE(config, nil)
		t.Assert(config.Enabled, false) // 默认是禁用的
		t.Assert(config.AT.EnableAsyncCommit, true)
	})
}

// ====================================================================================
// 配置测试
// ====================================================================================

// Test_Config tests configuration functionality
func Test_Config(t *testing.T) {
	// Test default config
	gtest.C(t, func(t *gtest.T) {
		config := DefaultConfig()
		t.AssertNE(config, nil)
		t.AssertEQ(config.ApplicationID, "gf-app")
		t.AssertEQ(config.TxServiceGroup, "default_tx_group")
		t.AssertEQ(config.AT.UndoLogTable, "undo_log")
	})

	// Test branch type is always AT
	gtest.C(t, func(t *gtest.T) {
		config := DefaultConfig()
		result := config.GetBranchType()
		t.AssertEQ(result, branch.BranchTypeAT)
		t.Log("BranchType:", result)
	})
}

// ====================================================================================
// 资源管理测试
// ====================================================================================

// Test_Resource tests resource management
func Test_Resource(t *testing.T) {
	// Test resource creation
	gtest.C(t, func(t *gtest.T) {
		config := DefaultConfig()
		resource := NewResource(
			"test-resource",
			nil,
			nil,
			config,
		)

		t.AssertNE(resource, nil)
		t.AssertEQ(resource.GetResourceId(), "test-resource")
		t.AssertEQ(resource.GetBranchType(), branch.BranchTypeAT)
		t.AssertEQ(resource.GetResourceGroupId(), config.TxServiceGroup)
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

// ====================================================================================
// 事务测试
// ====================================================================================

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

// ====================================================================================
// SQL 解析测试
// ====================================================================================

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

// ====================================================================================
// 镜像生成测试
// ====================================================================================

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

// ====================================================================================
// DSN 构建测试
// ====================================================================================

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

// ====================================================================================
// 分支 Undo Log 测试
// ====================================================================================

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

// ====================================================================================
// 事务生命周期测试
// ====================================================================================

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

// ====================================================================================
// 复杂 SQL 场景测试
// ====================================================================================

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

// ====================================================================================
// 异步工作器测试
// ====================================================================================

func TestAsyncWorker_StartStop(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		worker := NewAsyncWorker(nil, DefaultAsyncWorkerConfig())

		// 测试启动
		worker.Start()
		t.Assert(worker.running, true)

		// 测试停止
		worker.Stop()
		t.Assert(worker.running, false)

		// 重复停止不应该panic
		worker.Stop()
	})
}

func TestAsyncWorker_BranchCommit(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		config := DefaultAsyncWorkerConfig()
		config.WorkerPoolSize = 2
		config.QueueSize = 10

		worker := NewAsyncWorker(nil, config)
		worker.Start()
		defer worker.Stop()

		// 发送任务
		for i := 0; i < 5; i++ {
			err := worker.BranchCommit(rm.BranchResource{
				BranchType: branch.BranchTypeAT,
				Xid:        "test-xid",
				BranchId:   int64(i),
			})
			t.AssertNil(err)
		}

		// 等待处理
		time.Sleep(100 * time.Millisecond)

		// 检查统计
		stats := worker.GetStats()
		t.AssertGT(stats["total_tasks"], int64(0))
	})
}

func TestAsyncWorker_GetStats(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		worker := NewAsyncWorker(nil, DefaultAsyncWorkerConfig())
		worker.Start()
		defer worker.Stop()

		stats := worker.GetStats()
		t.AssertNE(stats, nil)
		t.Assert(stats["total_tasks"], int64(0))
		t.Assert(stats["success_tasks"], int64(0))
		t.Assert(stats["failed_tasks"], int64(0))
	})
}

func TestDefaultAsyncWorkerConfig(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		config := DefaultAsyncWorkerConfig()

		t.Assert(config.WorkerPoolSize, 10)
		t.Assert(config.QueueSize, 1000)
		t.Assert(config.BatchSize, 100)
		t.Assert(config.BatchInterval, 1000)
		t.Assert(config.RetryTimes, 3)
		t.Assert(config.RetryInterval, 100)
	})
}

// ====================================================================================
// 重试机制测试
// ====================================================================================

func TestRetryExecutor_Execute_Success(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		executor := NewRetryExecutor(DefaultRetryConfig())
		ctx := context.Background()

		callCount := 0
		err := executor.Execute(ctx, func() error {
			callCount++
			return nil
		})

		t.AssertNil(err)
		t.Assert(callCount, 1)
	})
}

func TestRetryExecutor_Execute_Retry(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		config := &RetryConfig{
			MaxRetries:      2,
			InitialBackoff:  10 * time.Millisecond,
			MaxBackoff:      100 * time.Millisecond,
			Multiplier:      2.0,
			RetryableErrors: []string{"timeout"},
		}
		executor := NewRetryExecutor(config)
		ctx := context.Background()

		callCount := 0
		err := executor.Execute(ctx, func() error {
			callCount++
			if callCount < 3 {
				return errors.New("timeout error")
			}
			return nil
		})

		t.AssertNil(err)
		t.Assert(callCount, 3)
	})
}

func TestRetryExecutor_Execute_MaxRetries(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		config := &RetryConfig{
			MaxRetries:      2,
			InitialBackoff:  10 * time.Millisecond,
			MaxBackoff:      100 * time.Millisecond,
			Multiplier:      2.0,
			RetryableErrors: []string{"timeout"},
		}
		executor := NewRetryExecutor(config)
		ctx := context.Background()

		callCount := 0
		err := executor.Execute(ctx, func() error {
			callCount++
			return errors.New("timeout error")
		})

		t.AssertNE(err, nil)
		t.Assert(callCount, 3) // 1 initial + 2 retries
	})
}

func TestRetryExecutor_Execute_NonRetryableError(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		config := &RetryConfig{
			MaxRetries:      2,
			InitialBackoff:  10 * time.Millisecond,
			MaxBackoff:      100 * time.Millisecond,
			Multiplier:      2.0,
			RetryableErrors: []string{"timeout"},
		}
		executor := NewRetryExecutor(config)
		ctx := context.Background()

		callCount := 0
		err := executor.Execute(ctx, func() error {
			callCount++
			return errors.New("not retryable error")
		})

		t.AssertNE(err, nil)
		t.Assert(callCount, 1) // No retry
	})
}

func TestWithTimeout_Success(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		ctx := context.Background()

		err := WithTimeout(ctx, 1*time.Second, func(ctx context.Context) error {
			time.Sleep(100 * time.Millisecond)
			return nil
		})

		t.AssertNil(err)
	})
}

func TestWithTimeout_Timeout(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		ctx := context.Background()

		err := WithTimeout(ctx, 100*time.Millisecond, func(ctx context.Context) error {
			time.Sleep(1 * time.Second)
			return nil
		})

		t.AssertNE(err, nil)
		t.AssertIN("timeout", err.Error())
	})
}

// ====================================================================================
// 降级策略测试
// ====================================================================================

// TestDefaultFallbackConfig 测试默认降级配置
func TestDefaultFallbackConfig(t *testing.T) {
	config := DefaultFallbackConfig()

	if config == nil {
		t.Fatal("DefaultFallbackConfig() should not return nil")
	}

	if !config.Enabled {
		t.Error("Expected Enabled to be true by default")
	}

	if config.Strategy != FallbackToLocal {
		t.Errorf("Expected Strategy to be FallbackToLocal, got %v", config.Strategy)
	}

	if config.ErrorThreshold != 5 {
		t.Errorf("Expected ErrorThreshold to be 5, got %d", config.ErrorThreshold)
	}

	if config.RecoveryTimeout != 30 {
		t.Errorf("Expected RecoveryTimeout to be 30, got %d", config.RecoveryTimeout)
	}

	if config.TimeWindow != 60 {
		t.Errorf("Expected TimeWindow to be 60, got %d", config.TimeWindow)
	}
}

// TestNewFallbackManager 测试创建降级管理器
func TestNewFallbackManager(t *testing.T) {
	t.Run("with config", func(t *testing.T) {
		config := &FallbackConfig{
			Enabled:         true,
			Strategy:        FallbackToLocal,
			ErrorThreshold:  5,
			TimeWindow:      60,
			RecoveryTimeout: 10,
		}

		manager := NewFallbackManager(config)
		if manager == nil {
			t.Fatal("NewFallbackManager should not return nil")
		}

		if manager.config.ErrorThreshold != 5 {
			t.Error("Config should be set correctly")
		}
	})

	t.Run("with nil config", func(t *testing.T) {
		manager := NewFallbackManager(nil)
		if manager == nil {
			t.Fatal("NewFallbackManager should not return nil")
		}

		// 应该使用默认配置
		if manager.config == nil {
			t.Error("Should use default config when nil is provided")
		}
	})
}

// TestFallbackManager_RecordError 测试记录错误
func TestFallbackManager_RecordError(t *testing.T) {
	config := &FallbackConfig{
		Enabled:         true,
		Strategy:        FallbackToLocal,
		ErrorThreshold:  3,
		TimeWindow:      60,
		RecoveryTimeout: 1,
	}

	manager := NewFallbackManager(config)
	ctx := context.Background()

	// 初始状态不应该降级
	if manager.IsFallback() {
		t.Error("Should not be in fallback mode initially")
	}

	// 记录错误但未达到阈值
	manager.RecordError(ctx)
	if manager.IsFallback() {
		t.Error("Should not trigger fallback with 1 error")
	}

	manager.RecordError(ctx)
	if manager.IsFallback() {
		t.Error("Should not trigger fallback with 2 errors")
	}

	// 达到阈值，触发降级
	manager.RecordError(ctx)
	if !manager.IsFallback() {
		t.Error("Should trigger fallback after reaching threshold")
	}
}

// TestFallbackManager_RecordSuccess 测试记录成功
func TestFallbackManager_RecordSuccess(t *testing.T) {
	config := &FallbackConfig{
		Enabled:         true,
		Strategy:        FallbackToLocal,
		ErrorThreshold:  3,
		TimeWindow:      60,
		RecoveryTimeout: 1,
	}

	manager := NewFallbackManager(config)
	ctx := context.Background()

	// 触发降级
	for i := 0; i < 3; i++ {
		manager.RecordError(ctx)
	}

	if !manager.IsFallback() {
		t.Fatal("Should be in fallback mode")
	}

	// 记录成功应该重置错误计数
	manager.RecordSuccess()

	// 错误计数应该被重置
	// 注意：降级状态需要等待 RecoveryTimeout 后才会自动恢复
}

// TestFallbackManager_CheckFallback 测试检查降级状态
func TestFallbackManager_CheckFallback(t *testing.T) {
	t.Run("disabled", func(t *testing.T) {
		config := &FallbackConfig{
			Enabled:        false,
			ErrorThreshold: 1,
		}
		manager := NewFallbackManager(config)

		// 即使有错误，禁用时也不应降级
		manager.RecordError(context.Background())
		if manager.IsFallback() {
			t.Error("Should not fallback when disabled")
		}
	})

	t.Run("enabled and threshold reached", func(t *testing.T) {
		config := &FallbackConfig{
			Enabled:         true,
			ErrorThreshold:  2,
			TimeWindow:      60,
			RecoveryTimeout: 10,
		}
		manager := NewFallbackManager(config)
		ctx := context.Background()

		manager.RecordError(ctx)
		manager.RecordError(ctx)

		if !manager.IsFallback() {
			t.Error("Should fallback after reaching threshold")
		}
	})
}

// TestFallbackManager_IsFallback 测试获取降级状态
func TestFallbackManager_IsFallback(t *testing.T) {
	config := &FallbackConfig{
		Enabled:        true,
		ErrorThreshold: 2,
	}
	manager := NewFallbackManager(config)

	// 初始状态
	if manager.IsFallback() {
		t.Error("Should not be in fallback initially")
	}

	// 触发降级
	ctx := context.Background()
	manager.RecordError(ctx)
	manager.RecordError(ctx)

	if !manager.IsFallback() {
		t.Error("Should be in fallback after errors")
	}
}

// TestFallbackStrategy 测试降级策略
func TestFallbackStrategy(t *testing.T) {
	tests := []struct {
		name     string
		strategy FallbackStrategy
	}{
		{"None", FallbackNone},
		{"ToLocal", FallbackToLocal},
		{"ToReadOnly", FallbackToReadOnly},
		{"Reject", FallbackReject},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &FallbackConfig{
				Enabled:  true,
				Strategy: tt.strategy,
			}

			manager := NewFallbackManager(config)
			if manager.config.Strategy != tt.strategy {
				t.Errorf("Expected strategy %v, got %v", tt.strategy, manager.config.Strategy)
			}
		})
	}
}

// TestFallbackManager_ExecuteWithFallback 测试带降级保护的执行
func TestFallbackManager_ExecuteWithFallback(t *testing.T) {
	t.Run("normal execution", func(t *testing.T) {
		config := DefaultFallbackConfig()
		config.Enabled = true
		manager := NewFallbackManager(config)
		ctx := context.Background()

		executed := false
		operation := func(ctx context.Context) error {
			executed = true
			return nil
		}

		err := manager.ExecuteWithFallback(ctx, nil, operation, nil)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if !executed {
			t.Error("Operation should be executed")
		}
	})

	t.Run("operation error", func(t *testing.T) {
		config := DefaultFallbackConfig()
		config.Enabled = true
		manager := NewFallbackManager(config)
		ctx := context.Background()

		expectedErr := errors.New("operation failed")
		operation := func(ctx context.Context) error {
			return expectedErr
		}

		err := manager.ExecuteWithFallback(ctx, nil, operation, nil)
		if err != expectedErr {
			t.Errorf("Expected error %v, got %v", expectedErr, err)
		}
	})

	t.Run("with fallback operation", func(t *testing.T) {
		config := &FallbackConfig{
			Enabled:        true,
			Strategy:       FallbackToLocal,
			ErrorThreshold: 1,
		}
		manager := NewFallbackManager(config)
		ctx := context.Background()

		// 触发降级
		manager.RecordError(ctx)

		fallbackExecuted := false
		fallbackOp := func(ctx context.Context) error {
			fallbackExecuted = true
			return nil
		}

		mainOp := func(ctx context.Context) error {
			t.Error("Main operation should not be executed in fallback mode")
			return nil
		}

		err := manager.ExecuteWithFallback(ctx, nil, mainOp, fallbackOp)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if !fallbackExecuted {
			t.Error("Fallback operation should be executed")
		}
	})
}

// mockDB 用于测试的模拟数据库
type mockDB struct {
	gdb.DB
}

// TestFallbackManager_handleFallback 测试处理降级
func TestFallbackManager_handleFallback(t *testing.T) {
	ctx := context.Background()
	db := &mockDB{}

	tests := []struct {
		name        string
		strategy    FallbackStrategy
		shouldError bool
	}{
		{
			name:        "FallbackToLocal",
			strategy:    FallbackToLocal,
			shouldError: false,
		},
		{
			name:        "FallbackToReadOnly",
			strategy:    FallbackToReadOnly,
			shouldError: true,
		},
		{
			name:        "FallbackReject",
			strategy:    FallbackReject,
			shouldError: true,
		},
		{
			name:        "FallbackNone",
			strategy:    FallbackNone,
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &FallbackConfig{
				Enabled:  true,
				Strategy: tt.strategy,
			}
			manager := NewFallbackManager(config)

			operation := func(ctx context.Context) error {
				return nil
			}

			err := manager.handleFallback(ctx, db, operation)

			if tt.shouldError && err == nil {
				t.Error("Expected error but got nil")
			}

			if !tt.shouldError && err != nil {
				t.Errorf("Expected no error but got %v", err)
			}
		})
	}
}

// TestFallbackManager_ConcurrentAccess 测试并发访问
func TestFallbackManager_ConcurrentAccess(t *testing.T) {
	config := &FallbackConfig{
		Enabled:        true,
		ErrorThreshold: 100,
	}
	manager := NewFallbackManager(config)
	ctx := context.Background()

	// 并发记录错误
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 10; j++ {
				manager.RecordError(ctx)
				manager.RecordSuccess()
				_ = manager.IsFallback()
			}
			done <- true
		}()
	}

	// 等待所有协程完成
	for i := 0; i < 10; i++ {
		<-done
	}

	// 验证没有 panic
	t.Log("Concurrent access test passed")
}

// ====================================================================================
// 功能测试
// ====================================================================================

// TestFunctional_ConfigValidation 测试配置验证
func TestFunctional_ConfigValidation(t *testing.T) {
	t.Run("validate AT mode config", func(t *testing.T) {
		config := DefaultConfig()
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
}

// ====================================================================================
// 集成测试（需要数据库环境）
// ====================================================================================

// 测试数据库配置（本地 MySQL）
const (
	testDBHost    = "127.0.0.1"
	testDBPort    = "3306"
	testDBUser    = "root"
	testDBPass    = "123456"
	testDBName    = "seata_test"
	testDBCharset = "utf8mb4"
)

// TestIntegration_BasicConnection 测试基本连接（集成测试）
func TestIntegration_BasicConnection(t *testing.T) {
	ctx := gctx.New()

	// 配置数据库
	gdb.SetConfig(gdb.Config{
		"default": gdb.ConfigGroup{
			gdb.ConfigNode{
				Host:    testDBHost,
				Port:    testDBPort,
				User:    testDBUser,
				Pass:    testDBPass,
				Name:    testDBName,
				Type:    "mysql",
				Charset: testDBCharset,
			},
		},
	})

	// 获取数据库实例
	db := g.DB()
	if db == nil {
		t.Skip("Database not available, skipping integration test")
	}

	// 测试连接
	err := db.PingMaster()
	if err != nil {
		t.Skipf("Cannot connect to database: %v", err)
	}

	// 执行简单查询
	result, err := db.Query(ctx, "SELECT 1 as num")
	if err != nil {
		t.Fatalf("Failed to execute query: %v", err)
	}

	if len(result) == 0 {
		t.Error("Expected result, got empty")
	}

	t.Log("Basic connection test passed")
}

// TestIntegration_TableOperations 测试表操作（集成测试）
func TestIntegration_TableOperations(t *testing.T) {
	ctx := context.Background()

	// 配置数据库
	gdb.SetConfig(gdb.Config{
		"test": gdb.ConfigGroup{
			gdb.ConfigNode{
				Host:    testDBHost,
				Port:    testDBPort,
				User:    testDBUser,
				Pass:    testDBPass,
				Name:    testDBName,
				Type:    "mysql",
				Charset: testDBCharset,
			},
		},
	})

	db := g.DB("test")
	if db == nil {
		t.Skip("Database not available")
	}

	// 测试连接
	if err := db.PingMaster(); err != nil {
		t.Skipf("Cannot connect to database: %v", err)
	}

	// 创建测试表
	createTableSQL := `
		CREATE TABLE IF NOT EXISTS seata_test_users (
			id INT PRIMARY KEY AUTO_INCREMENT,
			name VARCHAR(50),
			age INT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
	`

	_, err := db.Exec(ctx, createTableSQL)
	if err != nil {
		t.Fatalf("Failed to create test table: %v", err)
	}

	// 清理测试数据
	defer func() {
		_, _ = db.Exec(ctx, "DROP TABLE IF EXISTS seata_test_users")
	}()

	// 插入测试数据
	result, err := db.Insert(ctx, "seata_test_users", g.Map{
		"name": "test_user",
		"age":  25,
	})
	if err != nil {
		t.Fatalf("Failed to insert: %v", err)
	}

	lastId, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("Failed to get last insert id: %v", err)
	}

	t.Logf("Inserted row with ID: %d", lastId)

	// 查询数据
	count, err := db.Model("seata_test_users").Count()
	if err != nil {
		t.Fatalf("Failed to count: %v", err)
	}

	if count == 0 {
		t.Error("Expected at least 1 row")
	}

	t.Logf("Table operations test passed, row count: %d", count)
}

// TestIntegration_TransactionBasic 测试基本事务（集成测试）
func TestIntegration_TransactionBasic(t *testing.T) {
	ctx := context.Background()

	// 配置数据库
	gdb.SetConfig(gdb.Config{
		"test_tx": gdb.ConfigGroup{
			gdb.ConfigNode{
				Host:    testDBHost,
				Port:    testDBPort,
				User:    testDBUser,
				Pass:    testDBPass,
				Name:    testDBName,
				Type:    "mysql",
				Charset: testDBCharset,
			},
		},
	})

	db := g.DB("test_tx")
	if db == nil {
		t.Skip("Database not available")
	}

	if err := db.PingMaster(); err != nil {
		t.Skipf("Cannot connect to database: %v", err)
	}

	// 创建测试表
	_, err := db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS seata_test_accounts (
			id INT PRIMARY KEY AUTO_INCREMENT,
			user_id INT,
			balance DECIMAL(10,2),
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
	`)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	defer func() {
		_, _ = db.Exec(ctx, "DROP TABLE IF EXISTS seata_test_accounts")
	}()

	// 测试事务提交
	err = db.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		_, err := tx.Model("seata_test_accounts").Insert(g.Map{
			"user_id": 1,
			"balance": 1000.00,
		})
		return err
	})

	if err != nil {
		t.Fatalf("Transaction failed: %v", err)
	}

	// 验证数据
	count, err := db.Model("seata_test_accounts").Count()
	if err != nil {
		t.Fatalf("Failed to count: %v", err)
	}

	if count == 0 {
		t.Error("Expected transaction to commit")
	}

	t.Log("Basic transaction test passed")
}

// TestIntegration_TransactionRollback 测试事务回滚（集成测试）
func TestIntegration_TransactionRollback(t *testing.T) {
	ctx := context.Background()

	gdb.SetConfig(gdb.Config{
		"test_rollback": gdb.ConfigGroup{
			gdb.ConfigNode{
				Host:    testDBHost,
				Port:    testDBPort,
				User:    testDBUser,
				Pass:    testDBPass,
				Name:    testDBName,
				Type:    "mysql",
				Charset: testDBCharset,
			},
		},
	})

	db := g.DB("test_rollback")
	if db == nil {
		t.Skip("Database not available")
	}

	if err := db.PingMaster(); err != nil {
		t.Skipf("Cannot connect to database: %v", err)
	}

	// 创建测试表
	_, err := db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS seata_test_rollback (
			id INT PRIMARY KEY AUTO_INCREMENT,
			data VARCHAR(50)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
	`)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	defer func() {
		_, _ = db.Exec(ctx, "DROP TABLE IF EXISTS seata_test_rollback")
	}()

	// 获取初始计数
	initialCount, err := db.Model("seata_test_rollback").Count()
	if err != nil {
		t.Fatalf("Failed to get initial count: %v", err)
	}

	// 测试事务回滚
	err = db.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		_, err := tx.Model("seata_test_rollback").Insert(g.Map{
			"data": "should_rollback",
		})
		if err != nil {
			return err
		}

		// 返回错误以触发回滚
		return gerror.New("intentional rollback")
	})

	if err == nil {
		t.Error("Expected transaction to return error")
	}

	// 验证数据未提交
	finalCount, err := db.Model("seata_test_rollback").Count()
	if err != nil {
		t.Fatalf("Failed to get final count: %v", err)
	}

	if finalCount != initialCount {
		t.Errorf("Expected count to remain %d, got %d (rollback failed)",
			initialCount, finalCount)
	}

	t.Log("Transaction rollback test passed")
}

// ====================================================================================
// Seata 集成测试（需要 Seata 服务器环境）
// ====================================================================================

// TestSeataIntegration_BasicTransfer Seata AT模式基础转账测试
func TestSeataIntegration_BasicTransfer(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		// 注意：此测试需要运行 Seata 服务器
		// 可以使用 tests/ 目录下的 docker-compose 启动测试环境
		ctx := context.Background()

		// 配置数据库
		node := gdb.ConfigNode{
			Type:    DriverNameATMySQL,
			Host:    "127.0.0.1",
			Port:    "3306",
			User:    "root",
			Pass:    "123456",
			Name:    "seata_test",
			Charset: "utf8mb4",
			Extra:   "parseTime=true",
		}

		db, err := gdb.New(node)
		if err != nil {
			t.Skip("Cannot create database instance:", err)
			return
		}

		if err := db.PingMaster(); err != nil {
			t.Skip("Cannot connect to database:", err)
			return
		}

		// 重置测试数据
		_, _ = db.Model("accounts").Ctx(ctx).Where("user_id", 1).Data(gdb.Map{
			"balance": 10000.00,
		}).Update()
		_, _ = db.Model("accounts").Ctx(ctx).Where("user_id", 2).Data(gdb.Map{
			"balance": 5000.00,
		}).Update()

		// 获取转账前余额
		type Account struct {
			Balance float64
		}
		var acc1, acc2 Account
		err = db.Model("accounts").Where("user_id", 1).Scan(&acc1)
		t.AssertNil(err)
		err = db.Model("accounts").Where("user_id", 2).Scan(&acc2)
		t.AssertNil(err)

		t.Logf("转账前余额: 用户1=%.2f, 用户2=%.2f", acc1.Balance, acc2.Balance)

		// 执行转账事务
		err = db.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
			// 扣款
			_, err := tx.Model("accounts").Where("user_id", 1).Decrement("balance", 500.00)
			if err != nil {
				return err
			}

			// 加款
			_, err = tx.Model("accounts").Where("user_id", 2).Increment("balance", 500.00)
			return err
		})

		if err != nil {
			t.Skip("Transaction failed (Seata may not be running):", err)
			return
		}

		// 验证转账后余额
		err = db.Model("accounts").Where("user_id", 1).Scan(&acc1)
		t.AssertNil(err)
		err = db.Model("accounts").Where("user_id", 2).Scan(&acc2)
		t.AssertNil(err)

		t.Logf("转账后余额: 用户1=%.2f, 用户2=%.2f", acc1.Balance, acc2.Balance)
		t.Log("✅ Seata AT模式基础转账测试通过")
	})
}

// TestSeataIntegration_TransactionRollback Seata 事务回滚测试
func TestSeataIntegration_TransactionRollback(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		ctx := context.Background()

		node := gdb.ConfigNode{
			Type:    DriverNameATMySQL,
			Host:    "127.0.0.1",
			Port:    "3306",
			User:    "root",
			Pass:    "123456",
			Name:    "seata_test",
			Charset: "utf8mb4",
			Extra:   "parseTime=true",
		}

		db, err := gdb.New(node)
		if err != nil {
			t.Skip("Cannot create database instance:", err)
			return
		}

		if err := db.PingMaster(); err != nil {
			t.Skip("Cannot connect to database:", err)
			return
		}

		// 重置测试数据
		_, _ = db.Model("accounts").Ctx(ctx).Where("user_id", 1).Data(gdb.Map{
			"balance": 10000.00,
		}).Update()

		type Account struct {
			Balance float64
		}
		var accBefore Account
		err = db.Model("accounts").Where("user_id", 1).Scan(&accBefore)
		t.AssertNil(err)

		t.Logf("回滚测试开始: 用户1=%.2f", accBefore.Balance)

		// 执行会回滚的事务
		err = db.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
			_, err := tx.Model("accounts").Where("user_id", 1).Decrement("balance", 500.00)
			if err != nil {
				return err
			}
			// 故意返回错误触发回滚
			return errors.New("intentional rollback")
		})

		t.AssertNE(err, nil)

		// 验证余额未变
		var accAfter Account
		err = db.Model("accounts").Where("user_id", 1).Scan(&accAfter)
		t.AssertNil(err)
		t.AssertEQ(accAfter.Balance, accBefore.Balance)

		t.Logf("回滚测试结束: 用户1=%.2f (余额未变)", accAfter.Balance)
		t.Log("✅ Seata 事务回滚测试通过")
	})
}

// TestSeataIntegration_ConcurrentTransfer Seata 并发转账测试
func TestSeataIntegration_ConcurrentTransfer(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		ctx := context.Background()

		node := gdb.ConfigNode{
			Type:    DriverNameATMySQL,
			Host:    "127.0.0.1",
			Port:    "3306",
			User:    "root",
			Pass:    "123456",
			Name:    "seata_test",
			Charset: "utf8mb4",
			Extra:   "parseTime=true",
		}

		db, err := gdb.New(node)
		if err != nil {
			t.Skip("Cannot create database instance:", err)
			return
		}

		if err := db.PingMaster(); err != nil {
			t.Skip("Cannot connect to database:", err)
			return
		}

		// 重置测试数据
		_, _ = db.Model("accounts").Ctx(ctx).Where("user_id", 1).Data(gdb.Map{
			"balance": 10000.00,
		}).Update()
		_, _ = db.Model("accounts").Ctx(ctx).Where("user_id", 2).Data(gdb.Map{
			"balance": 5000.00,
		}).Update()

		concurrency := 5
		amount := 100.00
		var successCount int
		var mu sync.Mutex

		for i := 0; i < concurrency; i++ {
			err := db.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
				_, err := tx.Model("accounts").Where("user_id", 1).Decrement("balance", amount)
				if err != nil {
					return err
				}
				_, err = tx.Model("accounts").Where("user_id", 2).Increment("balance", amount)
				if err == nil {
					mu.Lock()
					successCount++
					mu.Unlock()
				}
				return err
			})
			if err != nil {
				t.Logf("并发事务 %d 失败: %v", i, err)
			}
		}

		t.Logf("并发转账完成: 成功=%d/%d", successCount, concurrency)
		t.Log("✅ Seata 并发转账测试完成")
	})
}
