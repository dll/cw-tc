package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	"github.com/cw-tc/server/internal/platform/clock"
	"github.com/gin-gonic/gin"
)

const requestIDHeader = "X-Request-ID"

const requestIDContextKey = "request_id"
const middlewareTraceKey = "middleware_trace"

func requestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(requestIDHeader)
		if id == "" {
			var raw [16]byte
			if _, err := rand.Read(raw[:]); err != nil {
				id = "unavailable"
			} else {
				id = hex.EncodeToString(raw[:])
			}
		}
		c.Header(requestIDHeader, id)
		c.Set(requestIDContextKey, id)
		appendMiddlewareTrace(c, "request_id")
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), requestIDContextKey, id))
		c.Next()
	}
}

func withClock(c *gin.Context) {
	c.Request = c.Request.WithContext(clock.With(c.Request.Context(), clock.Real()))
	c.Next()
}

func appendMiddlewareTrace(c *gin.Context, name string) {
	trace, _ := c.Get(middlewareTraceKey)
	items, _ := trace.([]string)
	c.Set(middlewareTraceKey, append(items, name))
}

func middlewareTrace(c *gin.Context) []string {
	trace, _ := c.Get(middlewareTraceKey)
	items, _ := trace.([]string)
	return append([]string(nil), items...)
}

func traceMiddleware(name string) gin.HandlerFunc {
	return func(c *gin.Context) {
		appendMiddlewareTrace(c, name)
		c.Next()
	}
}

// installMiddleware 提供批次 0 的可观察请求链。尚未实现的能力明确为 no-op，
// 后续接入真实实现时保持顺序不变。
func installMiddleware(r *gin.Engine, accessLog ...gin.HandlerFunc) {
	r.Use(requestID())
	r.Use(withClock)
	if len(accessLog) > 0 {
		r.Use(accessLog[0])
	} else {
		r.Use(traceMiddleware("access_log"))
	}
	for _, name := range []string{
		"rate_limit", "authentication", "ownership", "idempotency",
		"business", "serialization",
	} {
		r.Use(traceMiddleware(name))
	}
}
