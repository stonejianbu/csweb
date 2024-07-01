// Copyright 2024 huangyouguang <stonehuang90@gmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package csweb_utils

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Status struct {
	TraceId string `json:"traceId"`
	Code    int32  `json:"code"`
	Message string `json:"message"`
}

type ErrResp struct {
	Status Status `json:"status"`
}

// CustomErrorHandler custom Error Handler for HTTP Server
func CustomErrorHandler(ctx context.Context, mux *runtime.ServeMux, marshaler runtime.Marshaler, writer http.ResponseWriter, request *http.Request, err error) {
	s := status.Convert(err)
	pb := s.Proto()
	contentType := marshaler.ContentType(pb)
	writer.Header().Set("Content-Type", contentType)
	if s.Code() == codes.Unauthenticated {
		writer.Header().Set("WWW-Authenticate", s.Message())
	}
	traceId := ""
	if span := trace.SpanContextFromContext(request.Context()); span.IsSampled() {
		traceId = span.TraceID().String()
	}
	resp := ErrResp{
		Status: Status{
			TraceId: traceId,
			Code:    pb.GetCode(),
			Message: pb.GetMessage(),
		},
	}
	buf, err := marshaler.Marshal(resp)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		if _, err := io.WriteString(writer, `{"code": 13, "message": "failed to marshal error message"}`); err != nil {
			logrus.Infof("Failed to write response: %v", err)
		}
		return
	}
	// 获取原始请求头信息
	md, ok := runtime.ServerMetadataFromContext(ctx)
	if ok {
		// 透传请求头信息，修改header的key = Grpc-Metadata-<header-key>
		for k, vs := range md.HeaderMD {
			for _, v := range vs {
				writer.Header().Add(fmt.Sprintf("%s%s", runtime.MetadataHeaderPrefix, k), v)
			}
		}
	}
	// 写入http状态码，将code转换为http code
	st := runtime.HTTPStatusFromCode(s.Code())
	writer.WriteHeader(st)
	// 写入响应体
	if _, err := writer.Write(buf); err != nil {
		logrus.Infof("Failed to write response: %v", err)
	}
}
