package api

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type rateEntry struct {
	start time.Time
	count int
}

var (
	rateMu      sync.Mutex
	rateEntries = map[string]*rateEntry{}
)

// ipRateLimit 简单的单实例固定窗口限频（Redis 不可用时的兜底；生产多实例仍应接 Redis）。
func ipRateLimit(limit int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.FullPath() + "|" + c.ClientIP()
		now := time.Now()
		rateMu.Lock()
		entry, ok := rateEntries[key]
		if !ok || now.Sub(entry.start) >= window {
			entry = &rateEntry{start: now}
			rateEntries[key] = entry
		}
		entry.count++
		count := entry.count
		if len(rateEntries) > 20000 {
			for k, v := range rateEntries {
				if now.Sub(v.start) >= window {
					delete(rateEntries, k)
				}
			}
		}
		rateMu.Unlock()
		if count > limit {
			c.Header("Retry-After", strconv.Itoa(int(window.Seconds())))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, envelope{Code: 10903, Msg: "请求过于频繁，请稍后再试"})
			return
		}
		c.Next()
	}
}
