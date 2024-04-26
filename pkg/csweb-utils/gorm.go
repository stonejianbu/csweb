// Copyright 2024 huangyouguang <stonehuang90@gmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package csweb_utils

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const GormTrace = "gorm.trace"
const BeforeName = "gorm.before"
const AfterName = "gorm.after"

// NewGormDB 初始化DB
func NewGormDB(dsn string) (*gorm.DB, error) {
	newLogger := logger.Default
	newLogger.LogMode(logger.Info)
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: newLogger,
	})
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(100)
	sqlDB.SetMaxOpenConns(5000)
	sqlDB.SetConnMaxLifetime(time.Hour)
	gormTrace := NewGormTracer(db, "gorm.sql")
	if err := gormTrace.Init(); err != nil {
		return nil, err
	}
	return db, nil
}

// GormTracer gorm trace追踪
type GormTracer struct {
	db       *gorm.DB
	spanName string
}

func NewGormTracer(db *gorm.DB, spanName string) *GormTracer {
	return &GormTracer{
		db:       db,
		spanName: spanName,
	}
}

func (g *GormTracer) Init() error {
	err := g.db.Callback().Query().Before("*").Register(BeforeName, g.before)
	if err != nil {
		return err
	}
	err = g.db.Callback().Query().After("*").Register(AfterName, g.after)
	if err != nil {
		return err
	}
	err = g.db.Callback().Create().Before("*").Register(BeforeName, g.before)
	if err != nil {
		return err
	}
	err = g.db.Callback().Create().After("*").Register(AfterName, g.after)
	if err != nil {
		return err
	}
	err = g.db.Callback().Update().Before("*").Register(BeforeName, g.before)
	if err != nil {
		return err
	}
	err = g.db.Callback().Update().After("*").Register(AfterName, g.after)
	if err != nil {
		return err
	}
	err = g.db.Callback().Delete().Before("*").Register(BeforeName, g.before)
	if err != nil {
		return err
	}
	err = g.db.Callback().Delete().After("*").Register(AfterName, g.after)
	if err != nil {
		return err
	}
	err = g.db.Callback().Raw().Before("*").Register(BeforeName, g.before)
	if err != nil {
		return err
	}
	err = g.db.Callback().Raw().After("*").Register(AfterName, g.after)
	if err != nil {
		return err
	}
	err = g.db.Callback().Row().Before("*").Register(BeforeName, g.before)
	if err != nil {
		return err
	}
	err = g.db.Callback().Row().After("*").Register(AfterName, g.after)
	if err != nil {
		return err
	}
	return nil
}

func (g *GormTracer) before(scope *gorm.DB) {
	ctx, ok := scope.Get("ctx")
	if !ok {
		return
	}
	parentCtx, ok := ctx.(context.Context)
	if !ok {
		return
	}
	tracer := otel.GetTracerProvider().Tracer("gorm")
	_, span := tracer.Start(
		trace.ContextWithSpanContext(parentCtx, trace.SpanContextFromContext(parentCtx)),
		g.spanName,
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	scope.InstanceSet(GormTrace, span)
}

func (g *GormTracer) after(scope *gorm.DB) {
	ret, ok := scope.InstanceGet(GormTrace)
	if !ok {
		return
	}
	span, ok := ret.(trace.Span)
	if !ok {
		return
	}
	defer span.End()
	// 输出sql语句
	span.SetAttributes(attribute.String("sql", scope.Dialector.Explain(scope.Statement.SQL.String(), scope.Statement.Vars...)))
	if scope.Error != nil && scope.Error != gorm.ErrRecordNotFound {
		span.SetStatus(codes.Error, scope.Error.Error())
	}
}

type txKey struct{}

type optsKey struct{}

type ORMOption func(db *gorm.DB) *gorm.DB

func NewORMOptsContext(ctx context.Context, opts ...ORMOption) context.Context {
	return context.WithValue(ctx, optsKey{}, opts)
}

// ORMOptsFromContext 获取orm的options
func ORMOptsFromContext(ctx context.Context) []ORMOption {
	val := ctx.Value(optsKey{})
	if val == nil {
		return nil
	}
	return val.([]ORMOption)
}

// TxFromContext 从ctx获取事务连接
func TxFromContext(ctx context.Context) *gorm.DB {
	val := ctx.Value(txKey{})
	if val == nil {
		return nil
	}
	tx := val.(*gorm.DB)
	return tx
}

// NewTxContext ctx传递事务连接
func NewTxContext(ctx context.Context, db *gorm.DB) context.Context {
	return context.WithValue(ctx, txKey{}, db)
}

// CurrentDB 如果context里面有事务连接则使用事务连接，否则使用db
func CurrentDB(ctx context.Context, db *gorm.DB) *gorm.DB {
	db = db.Set("ctx", ctx) // 传递span ctx
	tx := TxFromContext(ctx)
	if tx != nil {
		return tx
	} else {
		return db
	}
}

// NotTxContext context清理事务DB
func NotTxContext(ctx context.Context) context.Context {
	return NewTxContext(ctx, nil)
}
