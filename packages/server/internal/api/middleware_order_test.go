package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMiddlewareChainOrderIsObservable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	installMiddleware(r)
	r.GET("/trace", func(c *gin.Context) {
		c.JSON(http.StatusOK, middlewareTrace(c))
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/trace", nil))

	want := []string{
		"request_id", "access_log", "rate_limit", "authentication",
		"ownership", "idempotency", "business", "serialization",
	}
	if got := middlewareTraceFromResponse(t, w); !reflect.DeepEqual(got, want) {
		t.Fatalf("中间件顺序 = %v，期望 %v", got, want)
	}
}

func middlewareTraceFromResponse(t *testing.T, w *httptest.ResponseRecorder) []string {
	t.Helper()
	var got []string
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("解析中间件顺序失败: %v; body=%s", err, w.Body.String())
	}
	return got
}
