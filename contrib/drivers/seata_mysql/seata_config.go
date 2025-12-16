// Copyright GoFrame Author(https://goframe.org). All Rights Reserved.
//
// This Source Code Form is subject to the terms of the MIT License.
// If a copy of the MIT was not distributed with this file,
// You can obtain one at https://github.com/gogf/gf.

package seata_mysql

import (
	"github.com/seata/seata-go/pkg/protocol/branch"
)

// Config Seata AT 模式配置结构
//
// 注意：本配置仅用于 GF 驱动层的控制，不用于 Seata 客户端配置。
// Seata 客户端配置请通过环境变量 SEATA_GO_CONFIG_PATH 指定配置文件。
type Config struct {
	// Enabled 是否启用 Seata AT 模式
	// false: 使用原生 MySQL 驱动，不启用分布式事务
	// true: 启用 Seata AT 模式分布式事务
	Enabled bool `json:"enabled" yaml:"enabled"`

	// AT AT 模式特定配置（仅用于精简模式下的 undo log 配置）
	AT ATConfig `json:"at" yaml:"at"`
}

// ATConfig AT 模式配置
//
// 注意：这些配置仅在精简模式（未设置 SEATA_GO_CONFIG_PATH）下使用。
// 完整模式下，所有配置从 seatago.yml 读取。
type ATConfig struct {
	// OnlyCarePrimaryKey 是否只关注主键
	// false: 记录所有列的变更（推荐，更安全）
	// true: 只记录主键列的变更（性能更好，但可能影响回滚准确性）
	OnlyCarePrimaryKey bool `json:"onlyCarePrimaryKey" yaml:"onlyCarePrimaryKey"`

	// EnableAsyncCommit 启用异步提交（异步删除 undo log）
	// true: 事务提交后异步删除 undo log（推荐，性能更好）
	// false: 事务提交时同步删除 undo log
	EnableAsyncCommit bool `json:"enableAsyncCommit" yaml:"enableAsyncCommit"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Enabled: false,
		AT: ATConfig{
			OnlyCarePrimaryKey: false,
			EnableAsyncCommit:  true,
		},
	}
}

// GetBranchType 获取分支类型（始终返回 AT）
func (c *Config) GetBranchType() branch.BranchType {
	return branch.BranchTypeAT
}
