// Copyright 2024 huangyouguang <stonehuang90@gmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package csweb_utils

import (
	"context"

	grpcprom "github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
)

var SrvMetrics = grpcprom.NewServerMetrics(
	grpcprom.WithServerHandlingTimeHistogram(
		grpcprom.WithHistogramBuckets([]float64{0.001, 0.01, 0.1, 0.3, 0.6, 1, 3, 6, 9, 20, 30, 60, 90, 120}),
	),
)

var PanicCounterMetrics = prometheus.NewCounter(prometheus.CounterOpts{
	Name: "grpc_req_panics_recovered_total",
	Help: "Total number of gRPC requests recovered from internal panic.",
})

func WithMetrics() grpc.UnaryServerInterceptor {
	return SrvMetrics.UnaryServerInterceptor(grpcprom.WithExemplarFromContext(func(ctx context.Context) prometheus.Labels {
		if span := trace.SpanContextFromContext(ctx); span.IsSampled() {
			return prometheus.Labels{
				"traceId": span.TraceID().String(),
				"spnId":   span.SpanID().String(),
			}
		}
		return nil
	}))
}

func InitMetrics() {
	prometheus.MustRegister(SrvMetrics, PanicCounterMetrics)
}
