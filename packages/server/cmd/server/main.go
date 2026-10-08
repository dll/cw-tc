// Command server 是第三课堂（cw-tc）的后端入口。
//
// 启动前先做配置校验：**强制密钥缺失即拒绝启动**（PRD v2 §12.2）。
// 这是刻意的"启动即失败"设计 —— 宁可起不来，也不能以降级方式
// 把身份证号退回到明文或弱密钥存储。
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cw-tc/server/internal/config"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "seed-admin" {
		if err := runSeedAdmin(); err != nil {
			os.Exit(1)
		}
		return
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel(),
	})))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("配置校验失败，服务拒绝启动", "err", err)
		os.Exit(1)
	}

	database, closeDatabase, err := openDatabase(context.Background(), cfg)
	if err != nil || database == nil {
		slog.Error("数据库初始化失败，服务拒绝启动", "err", err, "driver", cfg.DBDriver)
		os.Exit(1)
	}
	defer closeDatabase()

	// 能力开关如实播报，便于运维一眼看出"哪些功能因未配凭据而关闭"。
	slog.Info("配置已加载",
		"addr", cfg.Addr(),
		"smsEnabled", cfg.SMSEnabled(),
		"aiEnabled", cfg.LLMEnabled(),
		"wechatPayEnabled", cfg.WXPayEnabled(),
		"alipayEnabled", cfg.AlipayEnabled(),
		"publicStatusEnabled", cfg.PublicStatusEnabled,
		"logRedactEnabled", cfg.LogRedactEnabled,
	)

	srv := &http.Server{
		Addr:              cfg.Addr(),
		Handler:           buildAPI(cfg, database).Router(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		slog.Info("服务启动", "version", "dev")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("监听失败", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	slog.Info("收到退出信号，开始优雅关闭")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("优雅关闭超时", "err", err)
		os.Exit(1)
	}
	slog.Info("已退出")
}

// logLevel 从 LOG_LEVEL 解析日志级别，默认 info。
func logLevel() slog.Level {
	switch os.Getenv("LOG_LEVEL") {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
