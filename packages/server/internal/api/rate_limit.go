package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/cw-tc/server/internal/ratelimit"
)

var defaultRateLimiter = ratelimit.NewMemory()

// ipRateLimit 按路由 + 客户端 IP 限频；配置 Redis 后多实例共享计数。
func (s *Server) ipRateLimit(limit int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.FullPath() + "|" + c.ClientIP()
		limiter := s.rateLimiter
		if limiter == nil {
			limiter = defaultRateLimiter
		}
		allowed, err := limiter.Allow(c.Request.Context(), key, limit, window)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, envelope{Code: 10904, Msg: "限频服务暂不可用，请稍后再试"})
			return
		}
		if !allowed {
			c.Header("Retry-After", strconv.Itoa(int(window.Seconds())))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, envelope{Code: 10903, Msg: "请求过于频繁，请稍后再试"})
			return
		}
		c.Next()
	}
}
