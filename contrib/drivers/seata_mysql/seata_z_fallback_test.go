// Copyright GoFrame Author(https://goframe.org). All Rights Reserved.
//
// This Source Code Form is subject to the terms of the MIT License.
// If a copy of the MIT was not distributed with this file,
// You can obtain one at https://github.com/gogf/gf.

package seata_mysql

import (
	"context"
	"errors"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
)

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
