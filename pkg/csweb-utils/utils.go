// Copyright 2024 huangyouguang <stonehuang90@gmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package csweb_utils

import (
	"errors"
	"time"

	"github.com/stonejianbu/csweb/protos/common"
)

func GetFilterTime(start, end string) (startTime, endTime string, err error) {
	// 检查时间字符串的合法性
	if start != "" {
		_, err := time.Parse(time.DateOnly, start)
		if err != nil {
			return "", "", errors.New("startTime日期格式有误")
		}
	}
	if end != "" {
		_, err := time.Parse(time.DateOnly, end)
		if err != nil {
			return "", "", errors.New("endTime日期格式有误")
		}
	}
	// 如果时间字符串传入为空，则取最近三个月
	if start == "" {
		start = time.Now().AddDate(0, -3, 0).Format(time.DateOnly)
	}
	if end == "" {
		end = time.Now().Format(time.DateOnly)
	}
	// 起始时间不能大于终止时间
	s, _ := time.Parse(time.DateOnly, start)
	e, _ := time.Parse(time.DateOnly, end)
	if s.After(e) {
		return "", "", errors.New("startTime不能晚于endTime")
	}
	return start, end, nil
}

func CheckQueryPaging(paging *common.Page, defaultPageSize int32) *common.Page {
	if defaultPageSize <= 0 {
		defaultPageSize = 10
	}
	if paging == nil {
		paging = &common.Page{Page: 1, PerPage: defaultPageSize}
	} else {
		if paging.PerPage <= 0 {
			paging.PerPage = defaultPageSize
		}
		if paging.Page <= 0 {
			paging.Page = 1
		}
	}
	if paging.PerPage > 10000 {
		paging.PerPage = defaultPageSize
	}
	return paging
}

func GetResultPaging(query *common.Page, totalCount int32) (result *common.Page) {
	if query == nil {
		return nil
	}
	result = query
	result.TotalPage = totalCount / result.PerPage
	if totalCount%result.PerPage != 0 {
		result.TotalPage++
	}
	result.TotalRecord = totalCount
	return
}
