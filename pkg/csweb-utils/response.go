// Copyright 2024 huangyouguang <stonehuang90@gmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package csweb_utils

import (
	"context"

	"github.com/stonejianbu/csweb/protos/common"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func RespOK(ctx context.Context) *common.Response {
	return &common.Response{Status: OK(ctx)}
}

func OK(ctx context.Context) *common.Status {
	return &common.Status{
		TraceId: GetTraceId(ctx),
		Code:    int32(codes.OK),
		Message: codes.OK.String(),
	}
}

func InternalError(format string, a ...any) error {
	return status.Errorf(codes.Internal, format, a...)
}

func NotFoundError(err error) error {
	return status.Error(codes.NotFound, err.Error())
}

func QueryError(obj string) error {
	return status.Errorf(codes.Internal, "query %s failed, try again later", obj)
}

func UpdateError(obj string) error {
	return status.Errorf(codes.Internal, "update %s failed, try again later", obj)
}

func DeleteError(obj string) error {
	return status.Errorf(codes.Internal, "delete %s failed, try again later", obj)
}

func ArgumentError(format string, a ...any) error {
	return status.Errorf(codes.InvalidArgument, format, a...)
}

func PermissionDeniedError() error {
	return status.Errorf(codes.PermissionDenied, "PermissionDenied")
}
