package api

import (
	"net/http"
	"strings"
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

func TestAdminCertLedgerFilters(t *testing.T) {
	server, database, token, _ := adminDetailServer(t)
	defer database.Close()
	r := server.Router()

	all := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/certs", "", token)
	if all.Code != http.StatusOK || !containsAll(all.Body.String(), `"total":1`, `"cert_no":"CW-CN-A202600001"`, `"phone":"138****5678"`) {
		t.Fatalf("证书台账异常: %d %s", all.Code, all.Body.String())
	}
	if strings.Contains(all.Body.String(), "13812345678") {
		t.Fatalf("证书台账不得返回明文手机号: %s", all.Body.String())
	}
	byPrefix := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/certs?q=CW-CN-A2026", "", token)
	if byPrefix.Code != http.StatusOK || !containsAll(byPrefix.Body.String(), `"total":1`) {
		t.Fatalf("证书编号前缀筛选失败: %d %s", byPrefix.Code, byPrefix.Body.String())
	}
	byCourse := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/certs?course_code=C1&status=valid", "", token)
	if byCourse.Code != http.StatusOK || !containsAll(byCourse.Body.String(), `"total":1`) {
		t.Fatalf("课程 / 状态筛选失败: %d %s", byCourse.Code, byCourse.Body.String())
	}
	revoked := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/certs?status=revoked", "", token)
	if revoked.Code != http.StatusOK || !containsAll(revoked.Body.String(), `"total":0`) {
		t.Fatalf("已撤销筛选失败: %d %s", revoked.Code, revoked.Body.String())
	}
	for _, query := range []string{"?status=bad", "?limit=0", "?limit=101", "?offset=-1"} {
		if got := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/certs"+query, "", token); got.Code != http.StatusBadRequest {
			t.Fatalf("非法参数 %s 应 400，实际 %d", query, got.Code)
		}
	}
	// 撤销后按状态可查，并带撤销原因。
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/certs/CW-CN-A202600001/revoke",
		`{"reason":"实名不符","confirm":true}`, token); got.Code != http.StatusOK {
		t.Fatalf("撤销失败: %d", got.Code)
	}
	after := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/certs?status=revoked", "", token)
	if after.Code != http.StatusOK || !containsAll(after.Body.String(), `"status":"revoked"`, `"revoked_reason":"admin"`) {
		t.Fatalf("撤销后台账异常: %d %s", after.Code, after.Body.String())
	}
}
