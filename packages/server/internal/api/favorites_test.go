package api

import (
	"net/http"
	"strings"
	"testing"
)

func TestFavoritesLifecycleAndOwnership(t *testing.T) {
	s, token, cleanup := orderLifecycleServer(t)
	defer cleanup()
	r := s.Router()

	if got := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/courses/C1/favorite", `{}`, token); got.Code != http.StatusOK || !strings.Contains(got.Body.String(), `"favorited":true`) {
		t.Fatalf("收藏失败: %d %s", got.Code, got.Body.String())
	}
	if got := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/courses/C1/favorite", `{}`, token); got.Code != http.StatusOK {
		t.Fatalf("重复收藏应幂等: %d", got.Code)
	}
	status := performJSONWithCookie(t, r, http.MethodGet, "/api/v1/courses/C1/favorite", ``, token)
	if status.Code != http.StatusOK || !strings.Contains(status.Body.String(), `"favorited":true`) {
		t.Fatalf("收藏状态异常: %d %s", status.Code, status.Body.String())
	}
	list := performJSONWithCookie(t, r, http.MethodGet, "/api/v1/me/favorites", ``, token)
	if list.Code != http.StatusOK || !containsAll(list.Body.String(), `"course_code":"C1"`, `"name":"样例课程"`) {
		t.Fatalf("收藏列表异常: %d %s", list.Code, list.Body.String())
	}
	if got := performJSONWithCookie(t, r, http.MethodDelete, "/api/v1/courses/C1/favorite", ``, token); got.Code != http.StatusOK || !strings.Contains(got.Body.String(), `"favorited":false`) {
		t.Fatalf("取消收藏失败: %d %s", got.Code, got.Body.String())
	}
	if got := performJSONWithCookie(t, r, http.MethodGet, "/api/v1/me/favorites", ``, token); got.Code != http.StatusOK || strings.Contains(got.Body.String(), `"course_code":"C1"`) {
		t.Fatalf("取消后仍存在: %d %s", got.Code, got.Body.String())
	}
	if got := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/courses/NOPE/favorite", `{}`, token); got.Code != http.StatusNotFound {
		t.Fatalf("不存在课程应 404: %d", got.Code)
	}
	if got := performJSON(t, r, http.MethodGet, "/api/v1/me/favorites", ""); got.Code != http.StatusUnauthorized {
		t.Fatalf("未登录应 401: %d", got.Code)
	}
}
