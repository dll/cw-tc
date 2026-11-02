package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cw-tc/server/internal/identity"
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
	challenge := performJSON(t, r, http.MethodPost, "/api/v1/auth/captcha", `{}`)
	id := extractJSONField(t, challenge.Body.String(), "captcha_id")
	response := performJSON(t, r, http.MethodPost, "/api/v1/auth/sms/send", `{"phone":"13812345678","scene":"register","captcha_id":"`+id+`","captcha_answer":"test"}`)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("短信未配置响应 = %d %s", response.Code, response.Body.String())
	}
}

type recordingSMSSender struct {
	phone string
	code  string
}

func (r *recordingSMSSender) Send(_ context.Context, phone, code string) error {
	r.phone = phone
	r.code = code
	return nil
}

func TestSendSMSWithConfiguredProvider(t *testing.T) {
	s := New(testConfig(t, true)).WithSMS(identity.NewSMSService())
	sender := &recordingSMSSender{}
	s.smsSender = sender
	r := s.Router()
	challenge := performJSON(t, r, http.MethodPost, "/api/v1/auth/captcha", `{}`)
	id := extractJSONField(t, challenge.Body.String(), "captcha_id")
	response := performJSON(t, r, http.MethodPost, "/api/v1/auth/sms/send",
		`{"phone":"13812345678","scene":"login","captcha_id":"`+id+`","captcha_answer":"test"}`)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"code":0`) {
		t.Fatalf("短信发送失败: %d %s", response.Code, response.Body.String())
	}
	if sender.phone != "13812345678" || len(sender.code) != 6 {
		t.Fatalf("渠道参数异常: %+v", sender)
	}
	if err := s.sms.Consume("13812345678", "login", sender.code, timeNowUTC()); err != nil {
		t.Fatalf("验证码应可用于登录校验: %v", err)
	}
}
