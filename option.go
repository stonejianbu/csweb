// Copyright 2024 huangyouguang <stonehuang90@gmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package csweb

type Options struct {
	Gateway           string
	TraceAddr         string
	EnableMetrics     bool
	MetricsAddr       string
	JwtSignKey        string
	authFilterMethods []string
	RateLimit         int
}

type ServeOptions func(opts *Options)

func WithRateLimit(num int) ServeOptions {
	return func(opts *Options) {
		opts.RateLimit = num
	}
}

func WithJwtAuth(signKey string, authFilterMethods ...string) ServeOptions {
	return func(opts *Options) {
		opts.JwtSignKey = signKey
		opts.authFilterMethods = authFilterMethods
	}
}

func WithMetrics(addr string) ServeOptions {
	return func(opts *Options) {
		opts.MetricsAddr = addr
	}
}

func WithTracer(addr string) ServeOptions {
	return func(opts *Options) {
		opts.TraceAddr = addr
	}
}

// WithGateway enable the gateway and specify the address
func WithGateway(addr string) ServeOptions {
	return func(opts *Options) {
		opts.Gateway = addr
	}
}
