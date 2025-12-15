package seata_mysql

import (
	"testing"

	"github.com/gogf/gf/v2/test/gtest"

	"github.com/gogf/gf/contrib/drivers/mysql/v2"
)

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
		t.Assert(config.Mode, "AT")
		t.Assert(config.AT.EnableAsyncCommit, true)
	})
}
