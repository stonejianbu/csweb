// Copyright 2024 huangyouguang <stonehuang90@gmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package csweb_utils

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/auth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const OneDayTimestamp = 86400

var (
	TokenExpired     = errors.New("expired token")
	TokenNotValidYet = errors.New("unactivated token")
	TokenMalformed   = errors.New("unknown token")
	TokenInvalid     = errors.New("invalid token")
)

type JWT struct {
	SigningKey []byte
}

type CustomClaims struct {
	UID         string
	Username    string
	NickName    string
	AuthorityId string
	jwt.StandardClaims
}

func NewJWT(signKey string) *JWT {
	return &JWT{
		[]byte(signKey),
	}
}

// CreateToken 创建一个token
func (j *JWT) CreateToken(claims CustomClaims) (string, error) {
	claims.NotBefore = time.Now().Unix() - 100
	if claims.ExpiresAt == 0 {
		claims.ExpiresAt = time.Now().Unix() + OneDayTimestamp
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, &claims)
	return token.SignedString(j.SigningKey)
}

// ParseToken 解析 token
func (j *JWT) ParseToken(tokenString string) (*CustomClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(token *jwt.Token) (i interface{}, e error) {
		return j.SigningKey, nil
	})
	if err != nil {
		if ve, ok := err.(*jwt.ValidationError); ok {
			if ve.Errors&jwt.ValidationErrorMalformed != 0 {
				return nil, TokenMalformed
			} else if ve.Errors&jwt.ValidationErrorExpired != 0 {
				// Token is expired
				return nil, TokenExpired
			} else if ve.Errors&jwt.ValidationErrorNotValidYet != 0 {
				return nil, TokenNotValidYet
			} else {
				return nil, TokenInvalid
			}
		}
	}
	if token != nil {
		if claims, ok := token.Claims.(*CustomClaims); ok && token.Valid {
			return claims, nil
		}
		return nil, TokenInvalid

	} else {
		return nil, TokenInvalid

	}
}

var ClaimsKey struct{}

func SetClaimsWithContext(ctx context.Context, claims *CustomClaims) context.Context {
	return context.WithValue(ctx, ClaimsKey, claims)
}

func GetClaims(ctx context.Context) *CustomClaims {
	return ctx.Value(ClaimsKey).(*CustomClaims)
}

func GetUserName(ctx context.Context) string {
	return ctx.Value(ClaimsKey).(*CustomClaims).Username
}

// CookieToAuth Specifies that the value of the cookie key is converted to the value of the header Authorization
func CookieToAuth(cookieKey string) func(ctx context.Context, req *http.Request) metadata.MD {
	return func(ctx context.Context, req *http.Request) metadata.MD {
		var md metadata.MD
		// 优先使用Authorization，如果Authorization为空，则取cookie的token字段赋值给Authorization
		if len(req.Header.Get("Authorization")) != 0 {
			return md
		}
		tokenCookieKey := cookieKey
		for _, item := range strings.Split(req.Header.Get("Cookie"), ";") {
			temp := strings.Split(item, "=")
			if len(temp) == 2 {
				if temp[0] == tokenCookieKey {
					md = metadata.Pairs("Authorization", fmt.Sprintf("bearer %s", temp[1]))
				}
			}
		}
		return md
	}
}

func WithJwtAuth(signKey string, filterMethods ...string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if slices.Contains(filterMethods, info.FullMethod) {
			return handler(ctx, req)
		}
		var err error
		token, err := auth.AuthFromMD(ctx, "bearer")
		if err != nil {
			return nil, err
		}
		j := NewJWT(signKey)
		claims, err := j.ParseToken(token)
		if err != nil {
			return nil, err
		}
		ctx = SetClaimsWithContext(ctx, claims)
		return handler(ctx, req)
	}
}
