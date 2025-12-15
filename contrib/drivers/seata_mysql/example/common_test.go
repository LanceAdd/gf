// Copyright GoFrame Author(https://goframe.org). All Rights Reserved.
//
// This Source Code Form is subject to the terms of the MIT License.
// If a copy of the MIT was not distributed with this file,
// You can obtain one at https://github.com/gogf/gf.

package example

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/os/gctx"

	seata_mysql "github.com/gogf/gf/contrib/drivers/seata_mysql/v2"
)

var (
	// seataInitOnce 确保 Seata 只初始化一次
	seataInitOnce sync.Once
	// seataInitErr 保存初始化错误
	seataInitErr error
)

// setupSeataDB 初始化 Seata 数据库连接（AT 模式）
func setupSeataDB(t *testing.T, mode string) gdb.DB {
	ctx := gctx.GetInitCtx()

	// 使用 sync.Once 确保 Seata 只初始化一次
	seataInitOnce.Do(func() {
		// 设置 Seata 配置文件路径
		os.Setenv("SEATA_GO_CONFIG_PATH", "./seata.yml")

		// 初始化 Seata Driver
		config := &seata_mysql.Config{
			Enabled:        true,
			ApplicationID:  "gf-seata-test",
			TxServiceGroup: "default_tx_group",
			Mode:           mode,
			AT: seata_mysql.ATConfig{
				UndoLogTable: "undo_log",
			},
		}

		seataInitErr = seata_mysql.Init(config)
	})

	// 检查初始化是否成功
	if seataInitErr != nil {
		t.Fatalf("❌ Seata 初始化失败: %v", seataInitErr)
	}

	t.Logf("✅ Seata %s 模式已初始化", mode)

	// 直接创建数据库实例，不使用配置文件
	node := gdb.ConfigNode{
		Type:    seata_mysql.DriverNameATMySQL,
		Host:    "127.0.0.1",
		Port:    "3306",
		User:    "root",
		Pass:    "root",
		Name:    "seata_demo",
		Charset: "utf8mb4",
		Extra:   "parseTime=true",
	}

	db, err := gdb.New(node)
	if err != nil {
		t.Fatalf("❌ 创建数据库实例失败: %v", err)
	}

	// 测试连接
	err = db.PingMaster()
	if err != nil {
		t.Fatalf("❌ 数据库连接失败: %v", err)
	}

	_ = ctx // 避免 unused 警告
	return db
}

// resetAccountBalances 重置账户余额到初始状态（使用链式调用）
func resetAccountBalances(ctx context.Context, db gdb.DB) error {
	// 使用链式调用重置账户1
	_, err := db.Model("accounts").Ctx(ctx).Where("id", 1).Data(gdb.Map{
		"balance": 10000.00,
	}).Update()
	if err != nil {
		return err
	}

	// 使用链式调用重置账户2
	_, err = db.Model("accounts").Ctx(ctx).Where("id", 2).Data(gdb.Map{
		"balance": 5000.00,
	}).Update()
	return err
}

// resetProductStock 重置产品库存到初始状态（使用链式调用）
func resetProductStock(ctx context.Context, db gdb.DB) error {
	_, err := db.Model("products").Ctx(ctx).Where("id", 1).Data(gdb.Map{
		"stock": 100,
	}).Update()
	return err
}

// getAccountBalance 获取账户余额
func getAccountBalance(ctx context.Context, db gdb.DB, userID int) (float64, error) {
	type Account struct {
		Balance float64
	}
	var account Account
	err := db.Model("accounts").Where("id", userID).Scan(&account)
	if err != nil {
		return 0, err
	}
	return account.Balance, nil
}

// getProductStock 获取产品库存
func getProductStock(ctx context.Context, db gdb.DB, productID int) (int, error) {
	type Product struct {
		Stock int
	}
	var product Product
	err := db.Model("products").Where("id", productID).Scan(&product)
	if err != nil {
		return 0, err
	}
	return product.Stock, nil
}

// getProductPrice 获取产品价格
func getProductPrice(ctx context.Context, db gdb.DB, productID int) (float64, error) {
	type Product struct {
		Price float64
	}
	var product Product
	err := db.Model("products").Where("id", productID).Scan(&product)
	if err != nil {
		return 0, err
	}
	return product.Price, nil
}
