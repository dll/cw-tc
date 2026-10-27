// Package api 装配 HTTP 路由与处理器。
//
// 统一响应信封与 ccit 保持一致：{"code":0,"data":{...}} / 出错时 code != 0。
package api

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/cw-tc/server/internal/config"
	"github.com/cw-tc/server/internal/cosstore"
	"github.com/cw-tc/server/internal/identity"
	"github.com/cw-tc/server/internal/platform/clock"
	"github.com/cw-tc/server/internal/platform/db"
	"github.com/cw-tc/server/internal/redact"
)

// Version 为构建版本，发布时由 -ldflags 注入（见 Makefile）。
var Version = "dev"

// StartedAt 记录进程启动时间，用于健康检查报告运行时长。
var StartedAt = time.Now()

// Server 持有路由所需的依赖。
type Server struct {
	cfg      *config.Config
	identity *identity.Service
	database db.Pinger
	dbConn   *sql.DB
	sms      *identity.SMSService
	cosStore *cosstore.Store

	// COS 可用性探测缓存（欠费/停服时自动回退文件服务，见 cosAvailable）。
	cosProbeMu sync.Mutex
	cosProbes  map[string]cosProbeResult
}

type cosProbeResult struct {
	at time.Time
	ok bool
}

// New 构造 Server。
//
// ⚠️ **生产必须切到 `gin.ReleaseMode`** —— 否则服务跑在 debug 模式：打印路由表、
// 附带调试告警、性能略差（实测本机默认即为 debug）。开发环境保留 debug 便于排错。
func New(cfg *config.Config) *Server {
	if cfg != nil && cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}
	s := &Server{cfg: cfg}
	if cfg != nil {
		s.cosStore = cosstore.New(cfg.CosSecretID, cfg.CosSecretKey, cfg.CosBucket, cfg.CosRegion)
	}
	return s
}

type sqlPinger struct{ database *sql.DB }

func (p sqlPinger) Ping(ctx context.Context) error { return p.database.PingContext(ctx) }

func NewWithDatabase(cfg *config.Config, database *sql.DB) *Server {
	server := New(cfg)
	if database != nil {
		server.database = sqlPinger{database: database}
		server.dbConn = database
	}
	return server
}

func NewWithIdentityAndDatabase(cfg *config.Config, service *identity.Service, database *sql.DB) *Server {
	server := NewWithIdentity(cfg, service)
	if database != nil {
		server.database = sqlPinger{database: database}
		server.dbConn = database
	}
	return server
}

func NewWithIdentity(cfg *config.Config, service *identity.Service) *Server {
	server := New(cfg)
	server.identity = service
	return server
}

// WithSMS 注入短信服务，供验证码登录 / 找回 / 换绑使用；未注入时相关接口 fail-closed。
func (s *Server) WithSMS(sms *identity.SMSService) *Server {
	s.sms = sms
	if s.identity != nil {
		s.identity = s.identity.WithSMS(sms)
	}
	return s
}

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
	installMiddleware(r, s.accessLog())

	v1 := r.Group("/api/v1")
	v1.POST("/auth/captcha", s.issueCaptcha)
	v1.POST("/auth/captcha/verify", s.verifyCaptcha)
	v1.GET("/me", authRequired(s.identity), s.me)
	v1.POST("/me/password", authRequired(s.identity), s.changePassword)
	v1.GET("/me/entitlements", authRequired(s.identity), s.listEntitlements)
	v1.POST("/auth/sms/send", s.sendSMS)
	v1.POST("/auth/logout-all", s.logoutAll)
	v1.POST("/auth/logout", s.logout)
	v1.POST("/auth/login", s.login)
	v1.POST("/auth/sms-login", s.smsLogin)
	v1.POST("/auth/reset-password", s.resetPassword)
	v1.POST("/auth/register", s.register)
	v1.POST("/me/phone", authRequired(s.identity), s.rebindPhone)
	v1.POST("/me/delete", authRequired(s.identity), s.deleteAccount)
	v1.POST("/orders", authRequired(s.identity), s.createOrder)
	v1.GET("/orders/:orderNo", authRequired(s.identity), s.getOrder)
	v1.POST("/orders/:orderNo/pay", authRequired(s.identity), s.preparePayment)
	v1.POST("/orders/:orderNo/alipay/precreate", authRequired(s.identity), s.alipayPrecreate)
	v1.POST("/orders/:orderNo/mock-pay", authRequired(s.identity), s.mockPayOrder)
	v1.POST("/orders/:orderNo/cancel", authRequired(s.identity), s.cancelOrder)
	v1.POST("/exams", authRequired(s.identity), s.startExam)
	v1.POST("/exams/:sessionID/submit", authRequired(s.identity), s.submitExam)
	v1.GET("/exams/:sessionID", authRequired(s.identity), s.resumeExam)
	v1.POST("/exams/:sessionID/answers", authRequired(s.identity), s.saveExamAnswer)
	v1.GET("/me/notifications", authRequired(s.identity), s.listNotifications)
	v1.POST("/me/notifications/:id/read", authRequired(s.identity), s.markNotificationRead)
	v1.GET("/me/certs", authRequired(s.identity), s.listMyCerts)
	v1.GET("/me/certs/:certNo", authRequired(s.identity), s.downloadMyCert)
	v1.POST("/admin/login", s.adminLogin)
	admin := v1.Group("/admin", s.staffRequired)
	admin.POST("/change-password", s.adminChangePassword)
	admin.GET("/courses", s.adminCourses)
	admin.GET("/orders", s.adminOrders)
	admin.GET("/refunds", s.adminRefunds)
	admin.GET("/certs", s.adminCerts)
	admin.POST("/entitlements", s.adminGrantEntitlement)
	admin.POST("/student-discounts", s.requireAdmin, s.adminBatchDiscount)
	admin.POST("/student-grants", s.requireAdmin, s.adminBatchFree)
	admin.DELETE("/entitlements/:studentID/:courseID", s.adminRevokeEntitlement)
	admin.POST("/courses/:code/review", s.adminReviewCourse)
	admin.POST("/courses/:code/publish", s.adminPublishCourse)
	admin.POST("/courses/:code/price", s.adminSetPrice)
	admin.POST("/courses/:code/major", s.adminSetCourseMajor)
	admin.POST("/courses/:code/free-preview", s.adminSetCourseFreePreview)
	admin.POST("/broadcasts", s.adminCreateBroadcast)
	admin.POST("/broadcasts/:id/confirm", s.adminConfirmBroadcast)
	admin.POST("/staff", s.adminCreateStaff)
	admin.POST("/staff/:id/disable", s.adminDisableStaff)
	admin.POST("/courses/transfer", s.adminTransferCourse)
	admin.POST("/orders/:orderNo/refund", s.adminRefundOrder)
	admin.GET("/users", s.adminListUsers)
	admin.GET("/registration-pending", requireAdminOps(s), s.adminPendingRegistrations)
	admin.POST("/users/:id/reveal", s.adminRevealUser)
	admin.POST("/users/:id/approve-registration", requireAdminOps(s), s.adminApproveRegistration)
	admin.GET("/audit", s.adminAudit)
	admin.GET("/settings", s.requireSuper, s.adminSettings)
	admin.GET("/majors", s.adminListMajors)
	admin.POST("/majors", s.adminCreateMajor)
	admin.PUT("/majors/:id", s.adminUpdateMajor)
	teacher := v1.Group("/teacher", s.staffRequired)
	teacher.GET("/courses", s.teacherCourses)
	teacher.POST("/courses", s.teacherCreateCourse)
	teacher.PUT("/courses/:code", s.teacherUpdateCourse)
	teacher.DELETE("/courses/:code", s.teacherDeleteCourse)
	teacher.POST("/courses/:code/submit", s.teacherSubmitCourse)
	teacher.GET("/grade", s.teacherGradeList)
	teacher.GET("/grade/:resultID", s.teacherGradeDetail)
	teacher.POST("/grade/:resultID/questions/:questionID", s.teacherGradeSubmit)
	v1.POST("/payment/callback", s.paymentCallback)
	v1.POST("/pay/alipay/notify", s.alipayNotify)
	v1.GET("/courses", s.listPublicCourses)
	v1.GET("/courses/:courseCode", s.publicCourseDetail)
	v1.GET("/courses/:courseCode/lectures", s.listCourseLectures)
	v1.GET("/courses/:courseCode/lectures/:no/resources", authRequired(s.identity), s.listLectureResources)
	v1.GET("/courses/:courseCode/lectures/:no/practice", authRequired(s.identity), s.listLecturePractice)
	v1.POST("/courses/:courseCode/lectures/:no/practice/:questionID/answers", authRequired(s.identity), s.submitLectureAnswer)
	v1.GET("/courses/:courseCode/lectures/:no/topics", s.listLectureTopics)
	v1.POST("/courses/:courseCode/lectures/:no/topics", authRequired(s.identity), s.createLectureTopic)
	v1.GET("/courses/:courseCode/lectures/:no/topics/:topicID/replies", s.listLectureReplies)
	v1.POST("/courses/:courseCode/lectures/:no/topics/:topicID/replies", authRequired(s.identity), s.createLectureReply)
	v1.POST("/courses/:courseCode/lectures/:no/topics/:topicID/like", authRequired(s.identity), s.likeLectureTopic)
	v1.DELETE("/courses/:courseCode/lectures/:no/topics/:topicID/like", authRequired(s.identity), s.unlikeLectureTopic)
	v1.GET("/assets/raw", authRequired(s.identity), s.downloadAsset)
	v1.GET("/assets/media", s.streamSignedMedia)
	v1.GET("/assets/inline/:student/:asset/:expires/:sig/*relative", s.streamInlineAsset)
	v1.GET("/majors", s.listMajors)
	v1.GET("/health", s.health)
	v1.GET("/verify/:certNo", s.verifyCert)

	// 对外公开状态（PRD v2 §7.12 K-2）：只读、聚合、**不含任何个人信息**。
	// 未启用时路由不注册，避免"存在但不可用"的暧昧状态。
	if s.cfg.PublicStatusEnabled {
		v1.GET("/public/status", s.publicStatus)
	}
	return r
}

// health 健康检查。部署流水线的探活依赖（见 deploy/README.md）。
func (s *Server) health(c *gin.Context) {
	if s.database != nil {
		if err := s.database.Ping(c.Request.Context()); err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "数据库不可用"})
			return
		}
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{
		"status":     "ok",
		"version":    Version,
		"uptimeSec":  int64(time.Since(StartedAt).Seconds()),
		"observedAt": clock.From(c.Request.Context()).Now().Format(time.RFC3339),
	}})
}

// publicStatus 供 CCIT 官网 / AOC 读取的平台可用性快照。
//
// 硬约束（PRD v2 §7.12 K-2）：
//   - **不得**返回任何姓名、身份证号（含脱敏）、手机号、订单金额明细；
//   - 不可用时必须如实返回 unavailable，**不得伪造健康**。
//
// 具体聚合见 publicStatusSnapshot（public_status.go）：未接数据源时返回 null 而非 0
// —— 0 会被误读为"真的没有课程"，属铁律二禁止的编数；null 表达"未知"。
func (s *Server) publicStatus(c *gin.Context) {
	c.JSON(http.StatusOK, envelope{Code: 0, Data: s.publicStatusSnapshot(c.Request.Context())})
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
		appendMiddlewareTrace(c, "access_log")
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
