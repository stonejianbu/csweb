// Copyright 2024 huangyouguang <stonehuang90@gmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package csweb_utils

import (
	"context"
	"fmt"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
)

// LogWithContext get logger with fields(traceId,spanId)
func LogWithContext(ctx context.Context) *logrus.Entry {
	traceId := ""
	spanId := ""
	if span := trace.SpanContextFromContext(ctx); span.IsSampled() {
		traceId = span.TraceID().String()
		spanId = span.SpanID().String()
	}
	return logrus.WithFields(logrus.Fields{
		"traceId": traceId,
		"spanId":  spanId,
	})
}

// InterceptorLogger adapts logrus logger to interceptor logger.
func InterceptorLogger(l logrus.FieldLogger) logging.Logger {
	return logging.LoggerFunc(func(_ context.Context, lvl logging.Level, msg string, fields ...any) {
		f := make(map[string]any, len(fields)/2)
		i := logging.Fields(fields).Iterator()
		for i.Next() {
			k, v := i.At()
			f[k] = v
		}
		l := l.WithFields(f)

		switch lvl {
		case logging.LevelDebug:
			l.Debug(msg)
		case logging.LevelInfo:
			l.Info(msg)
		case logging.LevelWarn:
			l.Warn(msg)
		case logging.LevelError:
			l.Error(msg)
		default:
			panic(fmt.Sprintf("unknown level %v", lvl))
		}
	})
}

func WithLogger() grpc.UnaryServerInterceptor {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	opts := []logging.Option{
		logging.WithLogOnEvents(logging.StartCall, logging.PayloadReceived, logging.FinishCall),
		logging.WithFieldsFromContext(func(ctx context.Context) logging.Fields {
			if span := trace.SpanContextFromContext(ctx); span.IsSampled() {
				return logging.Fields{
					"traceId", span.TraceID().String(),
					"spanId", span.SpanID().String(),
				}
			}
			return logging.Fields{}
		}),
	}
	return logging.UnaryServerInterceptor(InterceptorLogger(logger), opts...)
}
