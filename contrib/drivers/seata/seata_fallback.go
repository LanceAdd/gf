// Copyright GoFrame Author(https://goframe.org). All Rights Reserved.
//
// This Source Code Form is subject to the terms of the MIT License.
// If a copy of the MIT was not distributed with this file,
// You can obtain one at https://github.com/gogf/gf.

package seata

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/os/gctx"
	"github.com/gogf/gf/v2/os/glog"
)

// FallbackStrategy 降级策略
type FallbackStrategy int

const (
	// FallbackNone 不降级
	FallbackNone FallbackStrategy = iota

	// FallbackToLocal 降级到本地事务
	FallbackToLocal

	// FallbackToReadOnly 降级到只读
	FallbackToReadOnly

	// FallbackReject 拒绝服务
	FallbackReject
)

// FallbackConfig 降级配置
type FallbackConfig struct {
	// Enabled 是否启用降级
	Enabled bool

	// Strategy 降级策略
	Strategy FallbackStrategy

	// ErrorThreshold 错误阈值（连续失败次数）
	ErrorThreshold int

	// TimeWindow 时间窗口（秒）
	TimeWindow int

	// RecoveryTimeout 恢复检测超时（秒）
	RecoveryTimeout int
}

// DefaultFallbackConfig 默认降级配置
func DefaultFallbackConfig() *FallbackConfig {
	return &FallbackConfig{
		Enabled:         true,
		Strategy:        FallbackToLocal,
		ErrorThreshold:  5,
		TimeWindow:      60,
		RecoveryTimeout: 30,
	}
}

// FallbackManager 降级管理器
type FallbackManager struct {
	config *FallbackConfig

	// 状态
	isFallback int32 // 0: 正常, 1: 降级

	// 统计
	errorCount    int32
	lastErrorTime atomic.Value // time.Time
	recoverTime   atomic.Value // time.Time
}

// NewFallbackManager 创建降级管理器
func NewFallbackManager(config *FallbackConfig) *FallbackManager {
	if config == nil {
		config = DefaultFallbackConfig()
	}

	fm := &FallbackManager{
		config: config,
	}

	fm.lastErrorTime.Store(time.Time{})
	fm.recoverTime.Store(time.Time{})

	// 启动恢复检测
	if config.Enabled {
		go fm.recoveryLoop()
	}

	return fm
}

// IsFallback 是否处于降级状态
func (fm *FallbackManager) IsFallback() bool {
	return atomic.LoadInt32(&fm.isFallback) == 1
}

// RecordError 记录错误
func (fm *FallbackManager) RecordError(ctx context.Context) {
	if !fm.config.Enabled {
		return
	}

	now := time.Now()
	lastError := fm.lastErrorTime.Load().(time.Time)

	// 检查是否在时间窗口内
	if !lastError.IsZero() && now.Sub(lastError) > time.Duration(fm.config.TimeWindow)*time.Second {
		// 超出时间窗口，重置计数
		atomic.StoreInt32(&fm.errorCount, 0)
	}

	// 增加错误计数
	count := atomic.AddInt32(&fm.errorCount, 1)
	fm.lastErrorTime.Store(now)

	// 检查是否达到降级阈值
	if count >= int32(fm.config.ErrorThreshold) && !fm.IsFallback() {
		fm.triggerFallback(ctx)
	}
}

// RecordSuccess 记录成功
func (fm *FallbackManager) RecordSuccess() {
	if !fm.config.Enabled {
		return
	}

	// 重置错误计数
	atomic.StoreInt32(&fm.errorCount, 0)
}

// triggerFallback 触发降级
func (fm *FallbackManager) triggerFallback(ctx context.Context) {
	atomic.StoreInt32(&fm.isFallback, 1)

	glog.Warningf(ctx, "[Fallback] Triggered fallback, strategy: %v, error count: %d",
		fm.getStrategyName(), atomic.LoadInt32(&fm.errorCount))

	// 记录降级时间
	fm.recoverTime.Store(time.Now().Add(time.Duration(fm.config.RecoveryTimeout) * time.Second))
}

// recoveryLoop 恢复检测循环
func (fm *FallbackManager) recoveryLoop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if !fm.IsFallback() {
			continue
		}

		recoverTime := fm.recoverTime.Load().(time.Time)
		if recoverTime.IsZero() {
			continue
		}

		// 检查是否到达恢复时间
		if time.Now().After(recoverTime) {
			fm.tryRecover()
		}
	}
}

// tryRecover 尝试恢复
func (fm *FallbackManager) tryRecover() {
	// 重置降级状态
	atomic.StoreInt32(&fm.isFallback, 0)
	atomic.StoreInt32(&fm.errorCount, 0)

	glog.Info(gctx.GetInitCtx(), "[Fallback] Recovered from fallback")
}

// getStrategyName 获取策略名称
func (fm *FallbackManager) getStrategyName() string {
	switch fm.config.Strategy {
	case FallbackNone:
		return "None"
	case FallbackToLocal:
		return "ToLocal"
	case FallbackToReadOnly:
		return "ToReadOnly"
	case FallbackReject:
		return "Reject"
	default:
		return "Unknown"
	}
}

// ExecuteWithFallback 带降级保护执行操作
func (fm *FallbackManager) ExecuteWithFallback(
	ctx context.Context,
	db gdb.DB,
	operation func(ctx context.Context) error,
	fallbackOperation func(ctx context.Context) error,
) error {
	// 检查是否需要降级
	if fm.IsFallback() {
		glog.Debugf(ctx, "[Fallback] In fallback mode, using fallback operation")

		if fallbackOperation != nil {
			err := fallbackOperation(ctx)
			if err == nil {
				fm.RecordSuccess()
			}
			return err
		}

		// 根据降级策略处理
		return fm.handleFallback(ctx, db, operation)
	}

	// 正常执行
	err := operation(ctx)
	if err != nil {
		fm.RecordError(ctx)
		return err
	}

	fm.RecordSuccess()
	return nil
}

// handleFallback 处理降级
func (fm *FallbackManager) handleFallback(
	ctx context.Context,
	db gdb.DB,
	operation func(ctx context.Context) error,
) error {
	switch fm.config.Strategy {
	case FallbackToLocal:
		// 降级到本地事务
		glog.Warning(ctx, "[Fallback] Degraded to local transaction")
		return operation(ctx)

	case FallbackToReadOnly:
		// 降级到只读
		glog.Warning(ctx, "[Fallback] Degraded to read-only mode")
		return gerror.NewCode(gcode.CodeNotSupported, "service degraded to read-only mode")

	case FallbackReject:
		// 拒绝服务
		glog.Warning(ctx, "[Fallback] Service rejected")
		return gerror.NewCode(gcode.CodeNotSupported, "service temporarily unavailable")

	default:
		return operation(ctx)
	}
}
