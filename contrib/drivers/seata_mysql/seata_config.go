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
type Config struct {
	// Enabled 是否启用 Seata
	Enabled bool `json:"enabled" yaml:"enabled"`

	// ApplicationID 应用 ID
	ApplicationID string `json:"applicationId" yaml:"applicationId"`

	// TxServiceGroup 事务服务组
	TxServiceGroup string `json:"txServiceGroup" yaml:"txServiceGroup"`

	// AT AT 模式配置
	AT ATConfig `json:"at" yaml:"at"`

	// Registry 注册中心配置
	Registry RegistryConfig `json:"registry" yaml:"registry"`

	// EnableAutoProxy 是否自动代理数据源
	EnableAutoProxy bool `json:"enableAutoProxy" yaml:"enableAutoProxy"`
}

// ATConfig AT 模式配置
type ATConfig struct {
	// UndoLogSerialization Undo Log 序列化方式
	UndoLogSerialization string `json:"undoLogSerialization" yaml:"undoLogSerialization"`

	// UndoLogTable Undo Log 表名
	UndoLogTable string `json:"undoLogTable" yaml:"undoLogTable"`

	// OnlyCarePrimaryKey 是否只关注主键
	OnlyCarePrimaryKey bool `json:"onlyCarePrimaryKey" yaml:"onlyCarePrimaryKey"`

	// EnableAsyncCommit 启用异步提交（异步删除 undo log）
	EnableAsyncCommit bool `json:"enableAsyncCommit" yaml:"enableAsyncCommit"`
}

// RegistryConfig 注册中心配置
type RegistryConfig struct {
	// Type 注册中心类型: nacos, eureka, consul, etcd, zk, sofa, redis, file
	Type string `json:"type" yaml:"type"`

	// FileConfig 文件注册中心配置
	FileConfig FileRegistryConfig `json:"file" yaml:"file"`

	// NacosConfig Nacos 配置
	NacosConfig NacosRegistryConfig `json:"nacos" yaml:"nacos"`
}

// FileRegistryConfig 文件注册中心配置
type FileRegistryConfig struct {
	// Name 文件名
	Name string `json:"name" yaml:"name"`
}

// NacosRegistryConfig Nacos 注册中心配置
type NacosRegistryConfig struct {
	// Application 应用名
	Application string `json:"application" yaml:"application"`

	// ServerAddr 服务地址
	ServerAddr string `json:"serverAddr" yaml:"serverAddr"`

	// Namespace 命名空间
	Namespace string `json:"namespace" yaml:"namespace"`

	// Group 分组
	Group string `json:"group" yaml:"group"`

	// Cluster 集群
	Cluster string `json:"cluster" yaml:"cluster"`

	// Username 用户名
	Username string `json:"username" yaml:"username"`

	// Password 密码
	Password string `json:"password" yaml:"password"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Enabled:         false,
		ApplicationID:   "gf-app",
		TxServiceGroup:  "default_tx_group",
		EnableAutoProxy: true,
		AT: ATConfig{
			UndoLogSerialization: "jackson",
			UndoLogTable:         "undo_log",
			OnlyCarePrimaryKey:   false,
			EnableAsyncCommit:    true, // 默认启用异步提交
		},
		Registry: RegistryConfig{
			Type: "file",
			FileConfig: FileRegistryConfig{
				Name: "registry.conf",
			},
		},
	}
}

// GetBranchType 获取分支类型（始终返回 AT）
func (c *Config) GetBranchType() branch.BranchType {
	return branch.BranchTypeAT
}
