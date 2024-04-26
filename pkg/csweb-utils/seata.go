// Copyright 2024 huangyouguang <stonehuang90@gmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package csweb_utils

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime/debug"
	"strconv"

	"github.com/seata/seata-go/pkg/constant"
	"github.com/seata/seata-go/pkg/tm"
	"google.golang.org/grpc/metadata"
	"gorm.io/gorm"
)

const txLogKey = "__tx_log"

type TxLogState int8

const (
	TxLogStateInit TxLogState = iota
	TxLogStateCommit
	TxLogStateRollback
)

type TxLog struct {
	Xid        string
	BranchId   string
	ActionName string
	LogDetail  []byte
	TxLogState TxLogState
	*gorm.Model
}

func PrepareFence(ctx context.Context, db *gorm.DB, prepare func(ctx context.Context, tx *gorm.DB) (txLogObj interface{}, ok bool, err error)) (ok bool, err error) {
	logger := LogWithContext(ctx)
	businessActionContext := tm.GetBusinessActionContext(ctx)
	if businessActionContext == nil {
		return false, errors.New("bad ctx without business action context")
	}
	xid := businessActionContext.Xid
	actionName := businessActionContext.ActionName
	brandId := businessActionContext.BranchId

	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("%s", string(debug.Stack()))
			err = errors.New("prepare failed")
		}
		if err != nil {
			tx.Rollback()
		} else {
			err = tx.Commit().Error
			ok = err == nil
		}
	}()
	logDetail, ok, err := prepare(ctx, tx)
	if err != nil {
		logger.Errorf("prepare fence failed, xid= %s, branchId= %d, [%v]", xid, brandId, err)
		return false, err
	}
	detail, err := json.Marshal(logDetail)
	if err != nil {
		err = fmt.Errorf("json marshal tx log detail failed, err: %s", err)
		return false, err
	}
	if detail == nil {
		detail = []byte{}
	}
	txLog := &TxLog{
		Xid:        xid,
		BranchId:   strconv.FormatInt(brandId, 10),
		ActionName: actionName,
		LogDetail:  detail,
		TxLogState: TxLogStateInit,
	}
	err = tx.Create(txLog).Error
	if err != nil {
		logger.Errorf("insert tcc fence record failed, xid= %s, branchId= %d, [%v]", xid, brandId, err)
		return false, err
	}
	logger.Infof("tcc prepare finished")
	return true, nil
}

func CommitFence(ctx context.Context, db *gorm.DB, confirm func(ctx context.Context, tx *gorm.DB) (ok bool, err error)) (ok bool, err error) {
	logger := LogWithContext(ctx)
	businessActionContext := tm.GetBusinessActionContext(ctx)
	if businessActionContext == nil {
		return false, errors.New("bad ctx without business action context")
	}

	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("%s", string(debug.Stack()))
			err = errors.New("commit failed")
		}
		if err != nil {
			tx.Rollback()
		} else {
			ok = tx.Commit().Error == nil
		}
	}()
	xid := businessActionContext.Xid
	branchId := strconv.FormatInt(businessActionContext.BranchId, 10)
	txLog := &TxLog{}
	err = tx.Set("gorm:query_option", "FOR UPDATE").First(txLog, "branch_id = ? and xid = ?", branchId, xid).Error
	if err != nil {
		logger.Errorf("get tx log failed, xid: %s, branch id: %s err: %s", xid, branchId, err.Error())
		return false, err
	}
	if txLog.TxLogState == TxLogStateCommit {
		return true, nil
	} else if txLog.TxLogState == TxLogStateRollback {
		return false, fmt.Errorf("unexpected tx log state rollback in phase confirm, xid: %s, branch id: %s", xid, branchId)
	}

	ok, err = confirm(context.WithValue(ctx, txLogKey, txLog), tx)
	if !ok {
		return
	}
	err = tx.Model(txLog).Where("branch_id = ? and xid = ?", branchId, xid).Updates(map[string]interface{}{
		"tx_log_state": TxLogStateCommit,
	}).Error
	if err != nil {
		ok = false
	}
	logger.Infof("tcc commit finished")
	return
}

func RollbackFence(ctx context.Context, db *gorm.DB, cancel func(ctx context.Context, tx *gorm.DB) (ok bool, err error)) (ok bool, err error) {
	logger := LogWithContext(ctx)
	businessActionContext := tm.GetBusinessActionContext(ctx)
	if businessActionContext == nil {
		return false, errors.New("bad ctx without business action context")
	}

	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("%s", string(debug.Stack()))
			err = errors.New("rollback failed")
			ok = false
		}
		if !ok {
			tx.Rollback()
		} else {
			ok = tx.Commit().Error == nil
		}
	}()
	xid := businessActionContext.Xid
	branchId := strconv.FormatInt(businessActionContext.BranchId, 10)
	txLog := &TxLog{}
	err = tx.Set("gorm:query_option", "FOR UPDATE").First(txLog, "branch_id = ? and xid = ?", branchId, xid).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			logger.Infof("rollback, tx log record not found")
			txLog := &TxLog{
				Xid:        xid,
				BranchId:   branchId,
				ActionName: businessActionContext.ActionName,
				TxLogState: TxLogStateRollback,
				LogDetail:  []byte{},
			}
			err = tx.Create(txLog).Error
			if err != nil {
				return false, err
			} else {
				return true, nil
			}
		}
		return false, fmt.Errorf("get tx log failed, err: %s", err.Error())
	}

	if txLog.TxLogState == TxLogStateRollback {
		return true, nil
	} else if txLog.TxLogState == TxLogStateCommit {
		return false, fmt.Errorf("unexpected tx log state confirm in phase cancel, xid: %s, branch id: %s", xid, branchId)
	}

	ok, err = cancel(context.WithValue(ctx, txLogKey, txLog), tx)
	if !ok {
		return false, err
	}

	err = tx.Model(txLog).Where("branch_id = ? and xid = ?", branchId, xid).Updates(map[string]interface{}{
		"tx_log_state": TxLogStateRollback,
	}).Error
	if err != nil {
		ok = false
	}

	ok = true
	logger.Infof("tcc rollback finished")
	return
}

func GetTxLogObj(ctx context.Context, txLogObj interface{}) (err error) {
	txLog, ok := ctx.Value(txLogKey).(*TxLog)
	if !ok {
		return fmt.Errorf("context %s not found", txLogKey)
	}
	err = json.Unmarshal(txLog.LogDetail, txLogObj)
	if err != nil {
		return fmt.Errorf("unmarshal tx log failed, log: %s, err: %s", string(txLog.LogDetail), err.Error())
	}
	return nil
}

func GetXidFromContext(ctx context.Context) string {
	var xid string
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	if slice := md.Get(constant.XidKey); slice != nil && len(slice) > 0 {
		xid = slice[0]
	}
	if xid == "" {
		if slice := md.Get(constant.XidKeyLowercase); slice != nil && len(slice) > 0 {
			xid = slice[0]
		}
	}
	return xid
}
