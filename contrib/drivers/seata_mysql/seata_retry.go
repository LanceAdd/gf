// Copyright GoFrame Author(https://goframe.org). All Rights Reserved.
//
// This Source Code Form is subject to the terms of the MIT License.
// If a copy of the MIT was not distributed with this file,
// You can obtain one at https://github.com/gogf/gf.

package seata_mysql

import (
	"context"
	"math"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/os/glog"
)

// RetryConfig 重试配置
type RetryConfig struct {
	// MaxRetries 最大重试次数
	MaxRetries int

	// InitialBackoff 初始退避时间
	InitialBackoff time.Duration

	// MaxBackoff 最大退避时间
	MaxBackoff time.Duration

	// Multiplier 退避倍数
	Multiplier float64

	// RetryableErrors 可重试的错误类型
	RetryableErrors []string
}

// DefaultRetryConfig 默认重试配置
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries:      3,
		InitialBackoff:  100 * time.Millisecond,
		MaxBackoff:      5 * time.Second,
		Multiplier:      2.0,
		RetryableErrors: []string{"timeout", "connection", "network"},
	}
}

// RetryExecutor 重试执行器
type RetryExecutor struct {
	config *RetryConfig
}

// NewRetryExecutor 创建重试执行器
func NewRetryExecutor(config *RetryConfig) *RetryExecutor {
	if config == nil {
		config = DefaultRetryConfig()
	}
	return &RetryExecutor{
		config: config,
	}
}

// Execute 执行带重试的操作
func (r *RetryExecutor) Execute(ctx context.Context, operation func() error) error {
	var lastErr error

	for attempt := 0; attempt <= r.config.MaxRetries; attempt++ {
		// 检查 context 是否已取消
		select {
		case <-ctx.Done():
			return gerror.Wrap(ctx.Err(), "retry cancelled by context")
		default:
		}

		// 执行操作
		err := operation()
		if err == nil {
			// 成功
			if attempt > 0 {
				glog.Infof(ctx, "[Retry] Operation succeeded after %d attempts", attempt)
			}
			return nil
		}

		lastErr = err

		// 检查是否可重试
		if !r.isRetryable(err) {
			glog.Warningf(ctx, "[Retry] Error is not retryable: %v", err)
			return err
		}

		// 最后一次尝试失败
		if attempt >= r.config.MaxRetries {
			glog.Errorf(ctx, "[Retry] Max retries (%d) reached, last error: %v",
				r.config.MaxRetries, err)
			return gerror.Wrapf(err, "max retries (%d) exceeded", r.config.MaxRetries)
		}

		// 计算退避时间
		backoff := r.calculateBackoff(attempt)
		glog.Warningf(ctx, "[Retry] Attempt %d/%d failed: %v, retrying in %v",
			attempt+1, r.config.MaxRetries+1, err, backoff)

		// 等待退避时间
		select {
		case <-ctx.Done():
			return gerror.Wrap(ctx.Err(), "retry cancelled by context during backoff")
		case <-time.After(backoff):
			// 继续重试
		}
	}

	return lastErr
}

// ExecuteWithResult 执行带重试的操作（有返回值）
func (r *RetryExecutor) ExecuteWithResult(ctx context.Context, operation func() (interface{}, error)) (interface{}, error) {
	var result interface{}
	err := r.Execute(ctx, func() error {
		var opErr error
		result, opErr = operation()
		return opErr
	})
	return result, err
}

// calculateBackoff 计算退避时间
func (r *RetryExecutor) calculateBackoff(attempt int) time.Duration {
	// 指数退避: initialBackoff * (multiplier ^ attempt)
	backoff := float64(r.config.InitialBackoff) * math.Pow(r.config.Multiplier, float64(attempt))

	// 不超过最大退避时间
	if backoff > float64(r.config.MaxBackoff) {
		backoff = float64(r.config.MaxBackoff)
	}

	return time.Duration(backoff)
}

// isRetryable 检查错误是否可重试
func (r *RetryExecutor) isRetryable(err error) bool {
	if err == nil {
		return false
	}

	errMsg := err.Error()

	// 检查是否包含可重试的错误类型
	for _, retryableErr := range r.config.RetryableErrors {
		if contains(errMsg, retryableErr) {
			return true
		}
	}

	return false
}

// TimeoutConfig 超时配置
type TimeoutConfig struct {
	// ConnectTimeout 连接超时
	ConnectTimeout time.Duration

	// QueryTimeout 查询超时
	QueryTimeout time.Duration

	// TxTimeout 事务超时
	TxTimeout time.Duration

	// BranchRegisterTimeout 分支注册超时
	BranchRegisterTimeout time.Duration

	// BranchReportTimeout 分支报告超时
	BranchReportTimeout time.Duration
}

// DefaultTimeoutConfig 默认超时配置
func DefaultTimeoutConfig() *TimeoutConfig {
	return &TimeoutConfig{
		ConnectTimeout:        5 * time.Second,
		QueryTimeout:          30 * time.Second,
		TxTimeout:             60 * time.Second,
		BranchRegisterTimeout: 10 * time.Second,
		BranchReportTimeout:   10 * time.Second,
	}
}

// WithTimeout 在超时 context 中执行操作
func WithTimeout(ctx context.Context, timeout time.Duration, operation func(context.Context) error) error {
	if timeout <= 0 {
		// 无超时限制
		return operation(ctx)
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 使用 channel 接收结果
	errChan := make(chan error, 1)

	go func() {
		errChan <- operation(ctx)
	}()

	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		return gerror.Wrapf(ctx.Err(), "operation timeout after %v", timeout)
	}
}

// WithTimeoutResult 在超时 context 中执行操作（有返回值）
func WithTimeoutResult(ctx context.Context, timeout time.Duration, operation func(context.Context) (interface{}, error)) (interface{}, error) {
	if timeout <= 0 {
		return operation(ctx)
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	type result struct {
		data interface{}
		err  error
	}

	resultChan := make(chan result, 1)

	go func() {
		data, err := operation(ctx)
		resultChan <- result{data: data, err: err}
	}()

	select {
	case res := <-resultChan:
		return res.data, res.err
	case <-ctx.Done():
		return nil, gerror.Wrapf(ctx.Err(), "operation timeout after %v", timeout)
	}
}
