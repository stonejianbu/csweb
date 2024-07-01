// Copyright 2024 huangyouguang <stonehuang90@gmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package csweb_utils

import (
	"context"
	"runtime/debug"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func recoverFrom(ctx context.Context, p any) error {
	if span := trace.SpanContextFromContext(ctx); span.IsSampled() {
		logrus.WithFields(logrus.Fields{
			"traceId": span.TraceID().String(),
			"spanId":  span.SpanID().String(),
		}).Errorf("%s", string(debug.Stack()))
	} else {
		logrus.Errorf("%s", string(debug.Stack()))
	}
	// count the panic
	PanicCounterMetrics.Inc()
	return status.Errorf(codes.Internal, "%s", p)
}

func WithRecovery() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (_ any, err error) {
		defer func() {
			if r := recover(); r != nil {
				err = recoverFrom(ctx, r)
			}
		}()

		return handler(ctx, req)
	}
}
