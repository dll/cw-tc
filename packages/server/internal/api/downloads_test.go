package api

import (
	"context"
	"net/http"
	"testing"
)

func TestDownloadGrantLifecycleAndUnlockRequirement(t *testing.T) {
	s, token, cleanup := orderLifecycleServer(t)
	defer cleanup()
	if _, err := s.dbConn.Exec(`INSERT INTO lecture(course_id, no, title) VALUES(1,1,'第一讲')`); err != nil {
		t.Fatal(err)
	}
	r := s.Router()

	// 未解锁 → 403
	if got := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/me/downloads",
		`{"course_code":"C1","lecture_no":1}`, token); got.Code != http.StatusForbidden {
		t.Fatalf("未解锁申请下载应 403，实际 %d %s", got.Code, got.Body.String())
	}

	if _, err := s.dbConn.Exec(`INSERT INTO entitlement(student_id, course_id, source, granted_at) VALUES(1,1,'free','2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	created := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/me/downloads",
		`{"course_code":"C1","lecture_no":1}`, token)
	if created.Code != http.StatusOK || !containsAll(created.Body.String(), `"status":"valid"`, `"remaining_days":30`) {
		t.Fatalf("签发下载授权失败: %d %s", created.Code, created.Body.String())
	}
	list := performJSONWithCookie(t, r, http.MethodGet, "/api/v1/me/downloads", ``, token)
	if list.Code != http.StatusOK || !containsAll(list.Body.String(), `"course_code":"C1"`, `"lecture_title":"第一讲"`) {
		t.Fatalf("下载清单异常: %d %s", list.Code, list.Body.String())
	}
	if got := performJSONWithCookie(t, r, http.MethodDelete, "/api/v1/me/downloads/1?course_code=C1", ``, token); got.Code != http.StatusOK {
		t.Fatalf("删除下载失败: %d %s", got.Code, got.Body.String())
	}
	if got := performJSONWithCookie(t, r, http.MethodDelete, "/api/v1/me/downloads/1?course_code=C1", ``, token); got.Code != http.StatusNotFound {
		t.Fatalf("重复删除应 404，实际 %d", got.Code)
	}
	if got := performJSON(t, r, http.MethodGet, "/api/v1/me/downloads", ""); got.Code != http.StatusUnauthorized {
		t.Fatalf("未登录应 401，实际 %d", got.Code)
	}
}

func TestOfflineGrantRevokedOnRefund(t *testing.T) {
	s, token, cleanup := orderLifecycleServer(t)
	defer cleanup()
	r := s.Router()
	ctx := context.Background()
	if _, err := s.dbConn.Exec(`INSERT INTO lecture(course_id, no, title) VALUES(1,1,'第一讲')`); err != nil {
		t.Fatal(err)
	}
	created := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/orders", `{"course_code":"C1"}`, token)
	orderNo := extractJSONField(t, created.Body.String(), "order_no")
	if got := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/orders/"+orderNo+"/mock-pay", `{}`, token); got.Code != http.StatusOK {
		t.Fatalf("模拟支付失败: %d %s", got.Code, got.Body.String())
	}
	if got := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/me/downloads",
		`{"course_code":"C1","lecture_no":1}`, token); got.Code != http.StatusOK {
		t.Fatalf("已购课程应可下载: %d %s", got.Code, got.Body.String())
	}
	if got := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/me/orders/"+orderNo+"/refund", `{"reason":"样例退款"}`, token); got.Code != http.StatusOK {
		t.Fatalf("退款失败: %d %s", got.Code, got.Body.String())
	}
	list := performJSONWithCookie(t, r, http.MethodGet, "/api/v1/me/downloads", ``, token)
	if list.Code != http.StatusOK || !containsAll(list.Body.String(), `"status":"revoked"`, `"revoked_reason":"order_refunded"`) {
		t.Fatalf("退款后下载授权应失效: %d %s", list.Code, list.Body.String())
	}
	var studentID int64
	if err := s.dbConn.QueryRowContext(ctx, `SELECT student_id FROM download_grant LIMIT 1`).Scan(&studentID); err != nil || studentID != 1 {
		t.Fatalf("下载授权未落库: %d %v", studentID, err)
	}
}
