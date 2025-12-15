// Copyright GoFrame Author(https://goframe.org). All Rights Reserved.
//
// This Source Code Form is subject to the terms of the MIT License.
// If a copy of the MIT was not distributed with this file,
// You can obtain one at https://github.com/gogf/gf.

package seata

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gogf/gf/v2/test/gtest"
)

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
