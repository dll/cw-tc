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
