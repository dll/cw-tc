package api

import (
	"net/http"
	"strings"
	"testing"
)

func TestSendSMSConsumesVerifiedCaptchaBeforeProviderCheck(t *testing.T) {
	r := New(testConfig(t, true)).Router()
	challenge := performJSON(t, r, http.MethodPost, "/api/v1/auth/captcha", `{}`)
	id := extractJSONField(t, challenge.Body.String(), "captcha_id")
	response := performJSON(t, r, http.MethodPost, "/api/v1/auth/sms/send", `{"phone":"13812345678","scene":"register","captcha_id":"`+id+`","captcha_answer":"test"}`)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("验证码正确且短信未配置时应返回 503，实际 %d %s", response.Code, response.Body.String())
	}
	second := performJSON(t, r, http.MethodPost, "/api/v1/auth/sms/send", `{"phone":"13812345678","scene":"register","captcha_id":"`+id+`","captcha_answer":"test"}`)
	if second.Code != http.StatusOK || !strings.Contains(second.Body.String(), `"code":10002`) {
		t.Fatalf("验证码不得重放: %d %s", second.Code, second.Body.String())
	}
}
