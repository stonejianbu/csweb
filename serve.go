// Copyright 2024 huangyouguang <stonehuang90@gmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package csweb

import (
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
)

// ServeInterface impl your server
type ServeInterface interface {
	GRPCServe(s *grpc.Server) error
	HTTPServe(mux *runtime.ServeMux, opts []grpc.DialOption) error
}
