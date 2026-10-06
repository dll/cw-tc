package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSendSMSRequiresCaptchaBeforeChannelCheck(t *testing.T) {
	r := New(testConfig(t, true)).Router()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/sms/send", strings.NewReader(`{"phone":"13812345678","scene":"register"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	r.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"code":10002`) {
		t.Fatalf("缺 captcha 响应 = %d %s", response.Code, response.Body.String())
	}
}

func TestSendSMSFailsClosedWhenProviderIsNotConfigured(t *testing.T) {
	r := New(testConfig(t, true)).Router()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/sms/send", strings.NewReader(`{"phone":"13812345678","scene":"register","captcha_id":"cap-1","captcha_answer":"ok"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	r.ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("短信未配置响应 = %d %s", response.Code, response.Body.String())
	}
}
