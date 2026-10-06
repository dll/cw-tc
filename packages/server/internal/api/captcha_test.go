package api

import (
	"net/http"
	"strings"
	"testing"
)

func TestCaptchaEndpointReturnsChallengeWithoutAnswer(t *testing.T) {
	r := New(testConfig(t, true)).Router()
	response := performJSON(t, r, http.MethodPost, "/api/v1/auth/captcha", `{}`)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"captcha_id"`) {
		t.Fatalf("验证码响应 = %d %s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), `"answer"`) {
		t.Fatal("验证码接口不得回传答案")
	}
}

func TestCaptchaRejectsWrongAnswerAndAcceptsCorrectAnswer(t *testing.T) {
	r := New(testConfig(t, true)).Router()
	first := performJSON(t, r, http.MethodPost, "/api/v1/auth/captcha", `{}`)
	id := extractJSONField(t, first.Body.String(), "captcha_id")
	wrong := performJSON(t, r, http.MethodPost, "/api/v1/auth/captcha/verify", `{"captcha_id":"`+id+`","answer":"wrong"}`)
	if wrong.Code != http.StatusOK || !strings.Contains(wrong.Body.String(), `"code":10002`) {
		t.Fatalf("错误答案响应 = %d %s", wrong.Code, wrong.Body.String())
	}
	correct := performJSON(t, r, http.MethodPost, "/api/v1/auth/captcha/verify", `{"captcha_id":"`+id+`","answer":"test"}`)
	if correct.Code != http.StatusOK {
		t.Fatalf("正确答案响应 = %d %s", correct.Code, correct.Body.String())
	}
}
