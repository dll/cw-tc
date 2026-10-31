package api

import (
	"net/http"
	"testing"
)

func TestAdminRevokeCertificate(t *testing.T) {
	server, database, token, _ := adminDetailServer(t)
	defer database.Close()
	r := server.Router()

	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/certs/NOPE/revoke",
		`{"reason":"错误签发","confirm":true}`, token); got.Code != http.StatusNotFound {
		t.Fatalf("不存在证书应 404，实际 %d", got.Code)
	}
	for _, body := range []string{
		`{"reason":"","confirm":true}`,
		`{"reason":"错误签发","confirm":false}`,
		`not-json`,
	} {
		if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/certs/CW-CN-A202600001/revoke", body, token); got.Code != http.StatusBadRequest {
			t.Fatalf("非法撤销请求应 400: %s -> %d", body, got.Code)
		}
	}
	ok := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/certs/CW-CN-A202600001/revoke",
		`{"reason":"学员实名不符，撤销证书","confirm":true}`, token)
	if ok.Code != http.StatusOK || !containsAll(ok.Body.String(), `"status":"revoked"`, `"revoked_reason":"admin"`) {
		t.Fatalf("撤销证书失败: %d %s", ok.Code, ok.Body.String())
	}
	var status, reason, revokedAt string
	var revokedBy int64
	if err := database.QueryRow(`SELECT status, revoked_reason, revoked_at, revoked_by FROM certificate WHERE cert_no='CW-CN-A202600001'`).
		Scan(&status, &reason, &revokedAt, &revokedBy); err != nil {
		t.Fatal(err)
	}
	if status != "revoked" || reason != "admin" || revokedAt == "" || revokedBy == 0 {
		t.Fatalf("撤销字段落库异常: %s %s %s %d", status, reason, revokedAt, revokedBy)
	}
	var notices, audits int
	if err := database.QueryRow(`SELECT COUNT(*) FROM notification WHERE student_id=1 AND type='cert' AND title='证书已撤销'`).Scan(&notices); err != nil || notices != 1 {
		t.Fatalf("撤销证书应通知学员: %d %v", notices, err)
	}
	if err := database.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action='cert.revoke'`).Scan(&audits); err != nil || audits != 1 {
		t.Fatalf("撤销证书必须写审计: %d %v", audits, err)
	}
	// 验真页应显示已撤销，而不是“不存在”。
	if verify := performJSON(t, r, http.MethodGet, "/api/v1/verify/CW-CN-A202600001", ""); verify.Code != http.StatusOK || !containsAll(verify.Body.String(), `"state":"revoked"`) {
		t.Fatalf("撤销后验真状态异常: %d %s", verify.Code, verify.Body.String())
	}
	// 重复撤销 → 409（编号不可重用、不可重复操作）。
	if again := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/certs/CW-CN-A202600001/revoke",
		`{"reason":"重复撤销","confirm":true}`, token); again.Code != http.StatusConflict {
		t.Fatalf("重复撤销应 409，实际 %d", again.Code)
	}
}
