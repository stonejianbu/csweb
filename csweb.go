// Copyright 2024 huangyouguang <stonehuang90@gmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package csweb

import (
	"context"
	"net"
	"net/http"
	"sync"
	"syscall"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/oklog/run"
	"github.com/pingcap/errors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"github.com/stonejianbu/csweb/pkg/csweb-utils"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var once sync.Once
var app *App

type App struct {
	Name  string
	Addr  string
	opts  *Options
	Serve ServeInterface
}

// NewApp new an app with name
func NewApp(name string, options ...ServeOptions) *App {
	once.Do(func() {
		opts := &Options{}
		for _, serveOpt := range options {
			serveOpt(opts)
		}
		app = &App{opts: opts, Name: name}
	})
	return app
}

// InitServe init the http serve and grpc serve
func (that *App) InitServe(s ServeInterface) {
	that.Serve = s
}

func (that *App) Run(addr string) error {
	that.Addr = addr
	if that.Serve == nil {
		return errors.New("Serve is nil, please call InitServe to init it")
	}
	g := &run.Group{}
	// init trace
	if err := csweb_utils.InitTracerProvider(that.Name, that.opts.TraceAddr); err != nil {
		logrus.Error(err)
	}
	// grpc server
	logrus.Infof("start to launch grpc server, listen at %s", that.Addr)
	that.startGrpcServer(g)
	// gateway server
	if len(that.opts.Gateway) > 0 {
		logrus.Infof("start to launch http server, listen at %s", that.opts.Gateway)
		that.startHttpServer(g)
	}
	// metrics server
	if len(that.opts.MetricsAddr) > 0 {
		logrus.Infof("start to launch http metrics server, listen at %s", that.opts.MetricsAddr)
		that.startMetricsServer(g)
	}
	// add signal handler
	g.Add(run.SignalHandler(context.Background(), syscall.SIGINT, syscall.SIGTERM))
	// start to running
	if err := g.Run(); err != nil {
		return err
	}
	return nil
}

func (that *App) startHttpServer(g *run.Group) {
	gatewayHttp := &http.Server{Addr: that.opts.Gateway}
	g.Add(func() error {
		mux := runtime.NewServeMux(
			runtime.WithErrorHandler(csweb_utils.CustomErrorHandler), // 错误Handler统一处理响应格式
			runtime.WithMetadata(csweb_utils.CookieToAuth("token")),  // 指定cookie的key的值转换为header Authorization的值
			runtime.WithOutgoingHeaderMatcher(func(key string) (string, bool) { // grpc设置的header透传出去，而不添加前缀Grpc-Metadata-
				return key, true
			}),
		)
		dialOpts := []grpc.DialOption{
			grpc.WithTransportCredentials(insecure.NewCredentials()), // disables transport security
			grpc.WithStatsHandler(otelgrpc.NewClientHandler()),       // trace
		}
		if err := that.Serve.HTTPServe(mux, dialOpts); err != nil {
			return err
		}
		gatewayHttp.Handler = csweb_utils.WithTrace(mux)
		gatewayHttp.Handler = Cors(gatewayHttp.Handler)
		return gatewayHttp.ListenAndServe()
	}, func(err error) {
		if err := gatewayHttp.Close(); err != nil {
			logrus.Errorf("failed to stop web server, err: %v", err)
		}
	})
}

func (that *App) startGrpcServer(g *run.Group) {
	usi := make([]grpc.UnaryServerInterceptor, 0)
	// logger
	usi = append(usi, csweb_utils.WithLogger())
	// ratelimit
	if that.opts.RateLimit != 0 {
		usi = append(usi, csweb_utils.WithRateLimit(that.opts.RateLimit))
	}
	// jwt auth
	if len(that.opts.JwtSignKey) > 0 {
		usi = append(usi, csweb_utils.WithJwtAuth(that.opts.JwtSignKey, that.opts.authFilterMethods...))
	}
	// metrics intercept
	usi = append(usi, csweb_utils.WithMetrics())
	// proto validator
	usi = append(usi, csweb_utils.WithValidator())
	// recover intercept
	usi = append(usi, csweb_utils.WithRecovery())
	// new grpc server instance
	grpcServer := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.ChainUnaryInterceptor(usi...),
		grpc.ChainStreamInterceptor(),
	)
	// async start grpc server
	g.Add(func() error {
		// register grpc server
		if err := that.Serve.GRPCServe(grpcServer); err != nil {
			return err
		}
		csweb_utils.SrvMetrics.InitializeMetrics(grpcServer)
		// start to listen
		listen, err := net.Listen("tcp", that.Addr)
		if err != nil {
			logrus.Error("net.Listen failed, err: %v", err)
			return err
		}
		if err := grpcServer.Serve(listen); err != nil {
			logrus.Errorf("server.Serve failed, err: %v", err)
		}
		return nil
	}, func(err error) {
		grpcServer.GracefulStop()
		return
	})
}

func (that *App) startMetricsServer(g *run.Group) {
	httpSrv := &http.Server{Addr: that.opts.MetricsAddr}
	g.Add(func() error {
		m := http.NewServeMux()
		csweb_utils.InitMetrics()
		m.Handle("/metrics", promhttp.Handler())
		httpSrv.Handler = m
		return httpSrv.ListenAndServe()
	}, func(err error) {
		_ = httpSrv.Close()
	})

}

func Cors(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := r.Method
		origin := r.Header.Get("Origin")
		if origin != "" {
			// 允许来源配置
			w.Header().Set("Access-Control-Allow-Origin", "*") // 可将将 * 替换为指定的域名
			w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, UPDATE, PATCH")
			w.Header().Set("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept, Authorization")
			w.Header().Set("Access-Control-Expose-Headers", "Content-Length, Access-Control-Allow-Origin, Access-Control-Allow-Headers, Cache-Control, Content-Language, Content-Type")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		if method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		handler.ServeHTTP(w, r)
	})
}
