package api

import (
	"context"
	"database/sql"
	"net/http"
	"testing"
	"time"
)

func TestAdminUserStatusSuspendBanUnban(t *testing.T) {
	server, database, superToken, _ := adminDetailServer(t)
	defer database.Close()
	r := server.Router()
	opsLogin := performJSON(t, r, http.MethodPost, "/api/v1/admin/login", `{"account":"ops","password":"Temp@123"}`)
	opsToken := extractJSONField(t, opsLogin.Body.String(), "token")
	until := time.Now().UTC().Add(72 * time.Hour).Format(time.RFC3339)
	if _, err := server.identity.CreateSession(context.Background(), 1, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}

	// ops 可临时停用，但必须填原因与期限。
	if got := performJSONWithBearer(t, r, http.MethodPatch, "/api/v1/admin/users/1/status",
		`{"status":"suspended","reason_kind":"payment_abuse","note":"恶意退款","until":"`+until+`","confirm":true}`, opsToken); got.Code != http.StatusOK {
		t.Fatalf("停用失败: %d %s", got.Code, got.Body.String())
	}
	var status, reason, note, untilDB string
	if err := database.QueryRow(`SELECT status, status_reason_kind, status_note, status_until FROM student WHERE id=1`).
		Scan(&status, &reason, &note, &untilDB); err != nil {
		t.Fatal(err)
	}
	if status != "suspended" || reason != "payment_abuse" || note != "恶意退款" || untilDB == "" {
		t.Fatalf("停用字段落库异常: %s %s %s %s", status, reason, note, untilDB)
	}
	var activeSessions int
	if err := database.QueryRow(`SELECT COUNT(*) FROM student_session WHERE student_id=1 AND revoked_at IS NULL`).Scan(&activeSessions); err != nil || activeSessions != 0 {
		t.Fatalf("停用后必须踢出全部会话: %d %v", activeSessions, err)
	}
	// 登录时必须看到原因类别与申诉入口。
	login := performJSON(t, r, http.MethodPost, "/api/v1/auth/login", `{"phone":"13812345678","password":"password-1"}`)
	if login.Code != http.StatusForbidden || !containsAll(login.Body.String(), `"code":10109`, `"reason_kind":"payment_abuse"`, `"appeal_url":"/auth/appeal"`) {
		t.Fatalf("停用登录提示异常: %d %s", login.Code, login.Body.String())
	}

	// ops 不能执行永久封禁。
	if got := performJSONWithBearer(t, r, http.MethodPatch, "/api/v1/admin/users/1/status",
		`{"status":"banned","reason_kind":"serious_violation","note":"倒卖课程","confirm":true}`, opsToken); got.Code != http.StatusForbidden {
		t.Fatalf("ops 永久封禁应 403，实际 %d", got.Code)
	}
	// super 可永久封禁。
	if got := performJSONWithBearer(t, r, http.MethodPatch, "/api/v1/admin/users/1/status",
		`{"status":"banned","reason_kind":"serious_violation","note":"倒卖课程","confirm":true}`, superToken); got.Code != http.StatusOK {
		t.Fatalf("永久封禁失败: %d %s", got.Code, got.Body.String())
	}
	var bannedUntil sql.NullString
	if err := database.QueryRow(`SELECT status, status_until FROM student WHERE id=1`).Scan(&status, &bannedUntil); err != nil || status != "banned" || bannedUntil.Valid {
		t.Fatalf("永久封禁落库异常: %s %+v %v", status, bannedUntil, err)
	}
	// 解封清空原因与期限。
	if got := performJSONWithBearer(t, r, http.MethodPatch, "/api/v1/admin/users/1/status",
		`{"status":"active","note":"申诉通过","confirm":true}`, opsToken); got.Code != http.StatusOK {
		t.Fatalf("解封失败: %d %s", got.Code, got.Body.String())
	}
	var reasonNull, untilNull any
	if err := database.QueryRow(`SELECT status_reason_kind, status_until FROM student WHERE id=1`).Scan(&reasonNull, &untilNull); err != nil {
		t.Fatal(err)
	}
	if reasonNull != nil || untilNull != nil {
		t.Fatalf("解封后原因/期限应清空: %v %v", reasonNull, untilNull)
	}
	var audits int
	if err := database.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action='user.status_change'`).Scan(&audits); err != nil || audits != 3 {
		t.Fatalf("状态变更必须写审计: %d %v", audits, err)
	}
}

func TestAdminUserStatusValidation(t *testing.T) {
	server, database, token, _ := adminDetailServer(t)
	defer database.Close()
	r := server.Router()
	cases := []string{
		`{"status":"suspended","reason_kind":"payment_abuse","note":"x","confirm":true}`,
		`{"status":"suspended","reason_kind":"bad","note":"x","until":"2030-01-01T00:00:00Z","confirm":true}`,
		`{"status":"suspended","reason_kind":"payment_abuse","note":"","until":"2030-01-01T00:00:00Z","confirm":true}`,
		`{"status":"banned","reason_kind":"serious_violation","note":"x","until":"2000-01-01T00:00:00Z","confirm":true}`,
		`{"status":"bad","confirm":true}`,
		`{"status":"suspended","reason_kind":"payment_abuse","note":"x","until":"2030-01-01T00:00:00Z","confirm":false}`,
	}
	for _, body := range cases {
		if got := performJSONWithBearer(t, r, http.MethodPatch, "/api/v1/admin/users/1/status", body, token); got.Code != http.StatusBadRequest {
			t.Fatalf("非法状态变更应 400: %s -> %d %s", body, got.Code, got.Body.String())
		}
	}
	if got := performJSONWithBearer(t, r, http.MethodPatch, "/api/v1/admin/users/999/status",
		`{"status":"active","confirm":true}`, token); got.Code != http.StatusNotFound {
		t.Fatalf("不存在用户应 404，实际 %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodPatch, "/api/v1/admin/users/bad/status", `{}`, token); got.Code != http.StatusBadRequest {
		t.Fatalf("非法用户号应 400，实际 %d", got.Code)
	}
	// 未登录 / 学员 token 不能调用。
	if got := performJSON(t, r, http.MethodPatch, "/api/v1/admin/users/1/status", `{}`); got.Code != http.StatusUnauthorized {
		t.Fatalf("未登录应 401，实际 %d", got.Code)
	}
}
