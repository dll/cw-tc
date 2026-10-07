package api

import (
	"net/http"
	"testing"
)

func TestSMSHandlerErrorBranches(t *testing.T) {
	s, sms, token, cleanup := smsAccountServer(t)
	defer cleanup()
	r := s.Router()

	// 参数缺失
	if resp := performJSON(t, r, http.MethodPost, "/api/v1/auth/sms-login", `{}`); resp.Code != http.StatusOK || !containsAll(resp.Body.String(), `"code":10002`) {
		t.Fatalf("缺参应返回参数错误: %d %s", resp.Code, resp.Body.String())
	}
	if resp := performJSON(t, r, http.MethodPost, "/api/v1/auth/reset-password", `{}`); resp.Code != http.StatusOK || !containsAll(resp.Body.String(), `"code":10002`) {
		t.Fatalf("缺参应返回参数错误: %d %s", resp.Code, resp.Body.String())
	}
	// 验证码错误
	if resp := performJSON(t, r, http.MethodPost, "/api/v1/auth/sms-login", `{"phone":"13812345678","code":"wrong1"}`); resp.Code != http.StatusUnauthorized {
		t.Fatalf("错误验证码应返回 401: %d %s", resp.Code, resp.Body.String())
	}
	if resp := performJSON(t, r, http.MethodPost, "/api/v1/auth/reset-password", `{"phone":"13812345678","code":"wrong1","new_password":"new-pass-1"}`); resp.Code != http.StatusUnauthorized {
		t.Fatalf("错误验证码应返回 401: %d %s", resp.Code, resp.Body.String())
	}
	// 换绑缺参
	if resp := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/me/phone", `{}`, token); resp.Code != http.StatusBadRequest {
		t.Fatalf("换绑缺参应返回 400: %d %s", resp.Code, resp.Body.String())
	}
	_ = sms
}
