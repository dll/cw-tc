package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRegisterEndpointRejectsMalformedJSON(t *testing.T) {
	r := New(testConfig(t, true)).Router()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader("{"))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	r.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"code":10002`) {
		t.Fatalf("非法 JSON 响应 = %d %s", response.Code, response.Body.String())
	}
}

func TestRegisterEndpointFailsClosedWithoutIdentityService(t *testing.T) {
	r := New(testConfig(t, true)).Router()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"name":"样例"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	r.ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("未配置账号服务状态码 = %d", response.Code)
	}
}
