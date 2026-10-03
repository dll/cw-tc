// Package api 装配 HTTP 路由与处理器。
//
// 统一响应信封与 ccit 保持一致：{"code":0,"data":{...}} / 出错时 code != 0。
package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/cw-tc/server/internal/config"
	"github.com/cw-tc/server/internal/redact"
)

// Version 为构建版本，发布时由 -ldflags 注入（见 Makefile）。
var Version = "dev"

// StartedAt 记录进程启动时间，用于健康检查报告运行时长。
var StartedAt = time.Now()

// Server 持有路由所需的依赖。
type Server struct {
	cfg *config.Config
}

// New 构造 Server。
func New(cfg *config.Config) *Server { return &Server{cfg: cfg} }

// envelope 为统一响应信封。
type envelope struct {
	Code int    `json:"code"`
	Data any    `json:"data,omitempty"`
	Msg  string `json:"msg,omitempty"`
}

// Router 构建并返回 gin 引擎。
func (s *Server) Router() *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(s.accessLog())

	v1 := r.Group("/api/v1")
	v1.GET("/health", s.health)

	// 对外公开状态（PRD v2 §7.12 K-2）：只读、聚合、**不含任何个人信息**。
	// 未启用时路由不注册，避免"存在但不可用"的暧昧状态。
	if s.cfg.PublicStatusEnabled {
		v1.GET("/public/status", s.publicStatus)
	}
	return r
}

// health 健康检查。部署流水线的探活依赖（见 deploy/README.md）。
func (s *Server) health(c *gin.Context) {
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{
		"status":     "ok",
		"version":    Version,
		"uptimeSec":  int64(time.Since(StartedAt).Seconds()),
		"observedAt": time.Now().Format(time.RFC3339),
	}})
}

// publicStatus 供 CCIT 官网 / AOC 读取的平台可用性快照。
//
// 硬约束（PRD v2 §7.12 K-2）：
//   - **不得**返回任何姓名、身份证号（含脱敏）、手机号、订单金额明细；
//   - 不可用时必须如实返回 unavailable，**不得伪造健康**。
//
// 关于计数：**尚未接入数据源**（M1 起接 PostgreSQL）。
// 因此这里返回 **null 而非 0** —— 0 是一个会被误读为"真的没有课程"的数字，
// 属于铁律二禁止的"编数"；null 表达的是"未知"。
// 接入数据源后，各字段改为从库中聚合查询，届时才填真实值。
func (s *Server) publicStatus(c *gin.Context) {
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{
		"source":     "cw-tc",
		"observedAt": time.Now().Format(time.RFC3339),
		"status":     statusAvailable,
		"dataSource": "not_wired", // 显式标注：计数尚未接入数据源
		"courses": gin.H{
			"total":     nil,
			"published": nil,
			"byMajor":   nil,
		},
		"certificates": gin.H{
			"issued":  nil,
			"revoked": nil,
		},
		"learners": gin.H{
			"registered": nil,
		},
	}})
}

// 状态取值（与 PRD v2 §7.12 K-2 一致）。
const (
	statusAvailable   = "available"
	statusDegraded    = "degraded"
	statusUnavailable = "unavailable"
)

// accessLog 记录访问日志，并对 URL 做敏感信息过滤。
//
// 为什么必须过滤路径：即便登录/注册走 POST body（PRD v2 §9.2 的硬要求），
// 只要有人在别处把身份证号拼进 query，日志就会留下明文。
// 这里做**最后一道兜底**，配合 deploy/scripts/scan-sensitive.sh 的静态扫描形成双层防护。
func (s *Server) accessLog() gin.HandlerFunc {
	logger := slog.Default()
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		raw := c.Request.URL.RequestURI()
		if s.cfg.LogRedactEnabled {
			raw = redact.String(raw)
		}
		logger.Info("http",
			"method", c.Request.Method,
			"uri", raw,
			"status", c.Writer.Status(),
			"costMs", time.Since(start).Milliseconds(),
		)
	}
}
