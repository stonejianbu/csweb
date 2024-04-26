// Copyright 2024 huangyouguang <stonehuang90@gmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package csweb_utils

import (
	"fmt"
	"testing"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/stretchr/testify/assert"
)

func TestJWT_CreateToken(t *testing.T) {
	jwtObj := NewJWT("hello")
	// 生成token
	token, err := jwtObj.CreateToken(CustomClaims{
		UID:         "123456",
		NickName:    "stonejianbu",
		Username:    "stone",
		AuthorityId: "1000",
		StandardClaims: jwt.StandardClaims{
			NotBefore: time.Now().Unix() - 1000,  // 签名生效时间
			ExpiresAt: time.Now().Unix() + 86400, // 过期时间 1天
			Issuer:    "qmPlus",                  // 签名的发行者
		},
	})
	assert.Nil(t, err)
	fmt.Printf("ret == %#v\n", token)

	// 解析token
	parseToken, err := jwtObj.ParseToken(token)
	if err != nil {
		return
	}
	assert.Nil(t, err)
	fmt.Printf("ret == %#v\n", parseToken)
}
