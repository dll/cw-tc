package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestIDMiddlewarePreservesOrGeneratesID(t *testing.T) {
	r := newTestRouter(t, true)
	for _, tc := range []struct{ name, input, want string }{
		{"preserve", "req-fixed", "req-fixed"},
		{"generate", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
			if tc.input != "" {
				req.Header.Set(requestIDHeader, tc.input)
			}
			r.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Fatalf("状态码 = %d", w.Code)
			}
			got := w.Header().Get(requestIDHeader)
			if got == "" {
				t.Fatal("响应必须包含 request id")
			}
			if tc.want != "" && got != tc.want {
				t.Fatalf("request id = %q，期望 %q", got, tc.want)
			}
		})
	}
}

func TestSecurityHeadersAndNoStore(t *testing.T) {
	r := newTestRouter(t, true)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/health", nil))
	for _, header := range []string{"X-Content-Type-Options", "Referrer-Policy", "X-Frame-Options", "Permissions-Policy"} {
		if w.Header().Get(header) == "" {
			t.Fatalf("缺少安全响应头 %s", header)
		}
	}
	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("nosniff 值异常: %s", w.Header().Get("X-Content-Type-Options"))
	}

	admin := httptest.NewRecorder()
	r.ServeHTTP(admin, httptest.NewRequest(http.MethodGet, "/api/v1/admin/courses", nil))
	if admin.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("后台接口应 no-store，实际 %q", admin.Header().Get("Cache-Control"))
	}
	public := httptest.NewRecorder()
	r.ServeHTTP(public, httptest.NewRequest(http.MethodGet, "/api/v1/courses", nil))
	if public.Header().Get("Cache-Control") == "no-store" {
		t.Fatal("公开课程接口不应被强制 no-store（可缓存）")
	}
}

func TestIPRateLimitOnVerify(t *testing.T) {
	r := newTestRouter(t, true)
	var last *httptest.ResponseRecorder
	for i := 0; i < 21; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/verify/CW-CN-M000000099", nil)
		req.RemoteAddr = "203.0.113.77:4321"
		r.ServeHTTP(w, req)
		last = w
	}
	if last.Code != http.StatusTooManyRequests {
		t.Fatalf("第 21 次应限频 429，实际 %d %s", last.Code, last.Body.String())
	}
	if last.Header().Get("Retry-After") == "" {
		t.Fatal("限频响应应带 Retry-After")
	}
}
