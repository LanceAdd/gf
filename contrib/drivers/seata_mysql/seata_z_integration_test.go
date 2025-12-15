// Copyright GoFrame Author(https://goframe.org). All Rights Reserved.
//
// This Source Code Form is subject to the terms of the MIT License.
// If a copy of the MIT was not distributed with this file,
// You can obtain one at https://github.com/gogf/gf.

package seata_mysql

import (
	"context"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gctx"
)

// 测试数据库配置（本地 MySQL）
const (
	testDBHost    = "127.0.0.1"
	testDBPort    = "3306"
	testDBUser    = "root"
	testDBPass    = "root"
	testDBName    = "test"
	testDBCharset = "utf8mb4"
)

// TestIntegration_BasicConnection 测试基本连接（集成测试）
func TestIntegration_BasicConnection(t *testing.T) {
	// 跳过集成测试，除非明确启用
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

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
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

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
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

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
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

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

// TestIntegration_MultipleConnections 测试多连接（集成测试）
func TestIntegration_MultipleConnections(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	// 配置多个数据库连接
	gdb.SetConfig(gdb.Config{
		"conn1": gdb.ConfigGroup{
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
		"conn2": gdb.ConfigGroup{
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

	db1 := g.DB("conn1")
	db2 := g.DB("conn2")

	if db1 == nil || db2 == nil {
		t.Skip("Database not available")
	}

	// 测试两个连接
	err1 := db1.PingMaster()
	err2 := db2.PingMaster()

	if err1 != nil || err2 != nil {
		t.Skipf("Cannot connect: err1=%v, err2=%v", err1, err2)
	}

	// 并发查询
	done := make(chan bool, 2)

	go func() {
		_, err := db1.Query(ctx, "SELECT 1")
		if err != nil {
			t.Errorf("Query on conn1 failed: %v", err)
		}
		done <- true
	}()

	go func() {
		_, err := db2.Query(ctx, "SELECT 2")
		if err != nil {
			t.Errorf("Query on conn2 failed: %v", err)
		}
		done <- true
	}()

	<-done
	<-done

	t.Log("Multiple connections test passed")
}
