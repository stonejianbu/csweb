// Copyright 2024 huangyouguang <stonehuang90@gmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package csweb_utils

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TokenBucket 令牌桶结构体
type TokenBucket struct {
	Num        int           // 当前桶中的令牌数据
	Size       int           // 木桶中令牌的容量
	Rate       time.Duration // 生成一个令牌所需要的时间
	UpdateTime time.Time     // 记录每次令牌桶更新时间
}

// NewTokenBucket 创建令牌桶
func NewTokenBucket(size int, rate time.Duration) *TokenBucket {
	return &TokenBucket{
		Num:        size,
		Size:       size,
		Rate:       rate,
		UpdateTime: time.Now(),
	}
}

// Limit 验证是否能获取一个令牌
func (that *TokenBucket) Limit(_ context.Context) bool {
	now := time.Now()
	// 如果与上次请求的时间间隔超过了token rate则增加令牌，最大令牌数不超过桶容量
	if that.UpdateTime.Add(that.Rate).Before(now) {
		that.Num += int(now.Sub(that.UpdateTime) / that.Rate)
		if that.Num > that.Size {
			that.Num = that.Size
		}
		// 更新last
		that.UpdateTime = now
	}
	if that.Num <= 0 {
		return true
	}
	that.Num--
	return false
}

// WithRateLimit return a new unary server interceptors that performs request rate limiting.
func WithRateLimit(num int) grpc.UnaryServerInterceptor {
	limiter := NewTokenBucket(num, time.Second)
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if limiter.Limit(ctx) {
			return nil, status.Errorf(codes.ResourceExhausted, "ratelimit rejected, please retry later")
		}
		return handler(ctx, req)
	}
}
