// Copyright 2024 huangyouguang <stonehuang90@gmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package csweb_utils

import (
	"context"
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	stdout "go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/exporters/zipkin"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

// InitTracerProvider init tracer Provider
func InitTracerProvider(name, addr string) error {
	var exporter sdktrace.SpanExporter
	var err error
	if len(addr) != 0 {
		exporter, err = zipkin.New(addr)
		if err != nil {
			return err
		}
	} else {
		exporter, err = stdout.New()
		if err != nil {
			return err
		}
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(name),
		)),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	return nil
}

// WithTrace inject traceId for customErrorHandler
func WithTrace(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if span := trace.SpanContextFromContext(ctx); !span.IsSampled() {
			attrs := make([]attribute.KeyValue, 0)
			attrs = append(attrs, attribute.String("method", r.Method))
			attrs = append(attrs, attribute.String("remoteAddr", r.RemoteAddr))
			attrs = append(attrs, attribute.String("userAgent", r.UserAgent()))
			attrs = append(attrs, attribute.String("url", r.URL.String()))
			tracer := otel.GetTracerProvider().Tracer("http")
			_, span := tracer.Start(
				ctx,
				r.URL.Path,
				trace.WithSpanKind(trace.SpanKindClient),
				trace.WithAttributes(attrs...),
			)
			defer span.End()
			ctx = trace.ContextWithSpan(ctx, span)
		}
		h.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetTraceId get traceId from context
func GetTraceId(ctx context.Context) string {
	traceId := ""
	if span := trace.SpanContextFromContext(ctx); span.IsSampled() {
		traceId = span.TraceID().String()
	}
	return traceId
}
