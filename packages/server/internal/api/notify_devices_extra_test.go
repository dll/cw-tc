package api

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestNotificationPrefsReadUpdateAndValidation(t *testing.T) {
	s, token, cleanup := orderLifecycleServer(t)
	defer cleanup()
	r := s.Router()

	get := performJSONWithCookie(t, r, http.MethodGet, "/api/v1/me/messages/prefs", ``, token)
	if get.Code != http.StatusOK || !containsAll(get.Body.String(), `"broadcast_opt_out":false`) {
		t.Fatalf("默认偏好读取失败: %d %s", get.Code, get.Body.String())
	}
	patch := performJSONWithCookie(t, r, http.MethodPatch, "/api/v1/me/messages/prefs", `{"broadcast_opt_out":true}`, token)
	if patch.Code != http.StatusOK || !containsAll(patch.Body.String(), `"broadcast_opt_out":true`) {
		t.Fatalf("更新偏好失败: %d %s", patch.Code, patch.Body.String())
	}
	// 系统消息与事务通知不可退订：接口只接受广播开关，缺字段即 400。
	if got := performJSONWithCookie(t, r, http.MethodPatch, "/api/v1/me/messages/prefs", `{}`, token); got.Code != http.StatusBadRequest {
		t.Fatalf("缺少开关应 400，实际 %d", got.Code)
	}
	if got := performJSONWithCookie(t, r, http.MethodGet, "/api/v1/me/messages/prefs", ``, ""); got.Code != http.StatusUnauthorized {
		t.Fatalf("未登录应 401，实际 %d", got.Code)
	}
}

func TestRevokeDeviceOwnershipAndValidation(t *testing.T) {
	s, token, cleanup := orderLifecycleServer(t)
	defer cleanup()
	r := s.Router()
	ctx := context.Background()

	if got := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/me/devices/bad/revoke", ``, token); got.Code != http.StatusBadRequest {
		t.Fatalf("非法设备号应 400，实际 %d", got.Code)
	}
	if got := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/me/devices/999999/revoke", ``, token); got.Code != http.StatusNotFound {
		t.Fatalf("不存在设备应 404，实际 %d", got.Code)
	}
	session, err := s.identity.CreateSession(ctx, 1, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	revoked := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/me/devices/"+itoa64(session.ID)+"/revoke", ``, token)
	if revoked.Code != http.StatusOK || !containsAll(revoked.Body.String(), `"revoked":true`) {
		t.Fatalf("踢出设备失败: %d %s", revoked.Code, revoked.Body.String())
	}
	// 被踢出的 token 立即失效。
	if got := performJSONWithCookie(t, r, http.MethodGet, "/api/v1/me", ``, session.Token); got.Code != http.StatusUnauthorized {
		t.Fatalf("被踢出 token 应失效，实际 %d", got.Code)
	}
}

func itoa64(v int64) string {
	if v == 0 {
		return "0"
	}
	buf := make([]byte, 0, 20)
	for v > 0 {
		buf = append([]byte{byte('0' + v%10)}, buf...)
		v /= 10
	}
	return string(buf)
}
