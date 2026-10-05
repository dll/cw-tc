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
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), requestIDContextKey, id))
		c.Next()
	}
}

func withClock(c *gin.Context) {
	c.Request = c.Request.WithContext(clock.With(c.Request.Context(), clock.Real()))
	c.Next()
}
