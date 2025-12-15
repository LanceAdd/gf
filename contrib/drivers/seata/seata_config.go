// Copyright GoFrame Author(https://goframe.org). All Rights Reserved.
//
// This Source Code Form is subject to the terms of the MIT License.
// If a copy of the MIT was not distributed with this file,
// You can obtain one at https://github.com/gogf/gf.

package seata

import (
	"time"

	"github.com/seata/seata-go/pkg/protocol/branch"
)

// Config Seata 配置结构
type Config struct {
	// Enabled 是否启用 Seata
	Enabled bool `json:"enabled" yaml:"enabled"`

	// ApplicationID 应用 ID
	ApplicationID string `json:"applicationId" yaml:"applicationId"`

	// TxServiceGroup 事务服务组
	TxServiceGroup string `json:"txServiceGroup" yaml:"txServiceGroup"`

	// Mode 事务模式: AT 或 XA
	Mode string `json:"mode" yaml:"mode"`

	// AT AT 模式配置
	AT ATConfig `json:"at" yaml:"at"`

	// XA XA 模式配置
	XA XAConfig `json:"xa" yaml:"xa"`

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

// XAConfig XA 模式配置
type XAConfig struct {
	// BranchExecutionTimeout 分支执行超时时间（毫秒）
	BranchExecutionTimeout int64 `json:"branchExecutionTimeout" yaml:"branchExecutionTimeout"`

	// ConnectionTimeout 连接超时时间（毫秒）
	ConnectionTimeout int64 `json:"connectionTimeout" yaml:"connectionTimeout"`
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
		Mode:            "AT",
		EnableAutoProxy: true,
		AT: ATConfig{
			UndoLogSerialization: "jackson",
			UndoLogTable:         "undo_log",
			OnlyCarePrimaryKey:   false,
			EnableAsyncCommit:    true, // 默认启用异步提交
		},
		XA: XAConfig{
			BranchExecutionTimeout: 60000,
			ConnectionTimeout:      30000,
		},
		Registry: RegistryConfig{
			Type: "file",
			FileConfig: FileRegistryConfig{
				Name: "registry.conf",
			},
		},
	}
}

// GetBranchType 根据模式获取分支类型
func (c *Config) GetBranchType() branch.BranchType {
	switch c.Mode {
	case "XA":
		return branch.BranchTypeXA
	case "AT":
		return branch.BranchTypeAT
	case "TCC":
		return branch.BranchTypeTCC
	case "SAGA":
		return branch.BranchTypeSAGA
	default:
		return branch.BranchTypeAT
	}
}

// GetTimeout 获取超时时间
func (c *Config) GetTimeout() time.Duration {
	if c.Mode == "XA" {
		return time.Duration(c.XA.BranchExecutionTimeout) * time.Millisecond
	}
	return 60 * time.Second
}
