package api

import (
	"net/http"
	"strings"
	"testing"
)

func TestCourseCommentsRequireEntitlementAndArePublic(t *testing.T) {
	s, token, cleanup := orderLifecycleServer(t)
	defer cleanup()
	r := s.Router()
	var studentID, courseID int64
	_ = s.dbConn.QueryRow(`SELECT id FROM student LIMIT 1`).Scan(&studentID)
	_ = s.dbConn.QueryRow(`SELECT id FROM course WHERE course_code='C1'`).Scan(&courseID)

	empty := performJSON(t, r, http.MethodGet, "/api/v1/courses/C1/comments", "")
	if empty.Code != http.StatusOK || !strings.Contains(empty.Body.String(), `"count":0`) {
		t.Fatalf("空评价异常: %d %s", empty.Code, empty.Body.String())
	}
	if got := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/courses/C1/comments", `{"rating":5,"body":"这是一条足够长的课程评价内容，用于验证权限与展示。"}`, token); got.Code != http.StatusForbidden {
		t.Fatalf("未解锁评价应 403: %d %s", got.Code, got.Body.String())
	}
	if _, err := s.dbConn.Exec(`INSERT INTO entitlement(student_id,course_id,source,granted_at) VALUES($1,$2,'free','2026-01-01T00:00:00Z')`, studentID, courseID); err != nil {
		t.Fatal(err)
	}
	if got := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/courses/C1/comments", `{"rating":5,"body":"联系我 13812345678 购买答案"}`, token); got.Code != http.StatusBadRequest {
		t.Fatalf("违规内容应 400: %d %s", got.Code, got.Body.String())
	}
	if got := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/courses/C1/comments", `{"rating":5,"body":"这是一条足够长的课程评价内容，用于验证权限与展示。"}`, token); got.Code != http.StatusOK {
		t.Fatalf("评价失败: %d %s", got.Code, got.Body.String())
	}
	if got := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/courses/C1/comments", `{"rating":4}`, token); got.Code != http.StatusConflict {
		t.Fatalf("重复评价应 409: %d", got.Code)
	}
	list := performJSON(t, r, http.MethodGet, "/api/v1/courses/C1/comments", "")
	if list.Code != http.StatusOK || !containsAll(list.Body.String(), `"count":1`, `"nickname":"学员***"`, `"rating":5`) {
		t.Fatalf("评价列表异常: %d %s", list.Code, list.Body.String())
	}
}
