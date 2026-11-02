package api

import (
	"net/http"
	"strings"
	"testing"
)

func TestAdminEntitlementLedgerFilters(t *testing.T) {
	server, database, token, _ := adminDetailServer(t)
	defer database.Close()
	r := server.Router()

	list := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/entitlements?student_id=1&status=active", "", token)
	if list.Code != http.StatusOK || !containsAll(list.Body.String(), `"course_code":"C1"`, `"source":"free"`, `"status":"active"`) {
		t.Fatalf("权益台账异常: %d %s", list.Code, list.Body.String())
	}
	if strings.Contains(list.Body.String(), "13812345678") || strings.Contains(list.Body.String(), "110101199001011237") {
		t.Fatalf("权益台账不得返回手机号 / 身份证号: %s", list.Body.String())
	}
	byCourse := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/entitlements?course_code=C1&source=free", "", token)
	if byCourse.Code != http.StatusOK || !containsAll(byCourse.Body.String(), `"total":1`) {
		t.Fatalf("课程 / 来源筛选失败: %d %s", byCourse.Code, byCourse.Body.String())
	}
	empty := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/entitlements?status=revoked", "", token)
	if empty.Code != http.StatusOK || !containsAll(empty.Body.String(), `"total":0`) {
		t.Fatalf("已撤销筛选失败: %d %s", empty.Code, empty.Body.String())
	}
	for _, query := range []string{"?status=bad", "?source=bad", "?student_id=bad", "?limit=0", "?limit=101", "?offset=-1"} {
		if got := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/entitlements"+query, "", token); got.Code != http.StatusBadRequest {
			t.Fatalf("非法筛选 %s 应 400，实际 %d", query, got.Code)
		}
	}
}

func TestAdminRevokeEntitlementKeepsCertificate(t *testing.T) {
	server, database, token, _ := adminDetailServer(t)
	defer database.Close()
	r := server.Router()
	if got := performJSONWithBearer(t, r, http.MethodDelete, "/api/v1/admin/entitlements/1",
		`{"reason":"","confirm":true}`, token); got.Code != http.StatusBadRequest {
		t.Fatalf("缺少事由应 400，实际 %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodDelete, "/api/v1/admin/entitlements/1",
		`{"reason":"违规获取","confirm":false}`, token); got.Code != http.StatusBadRequest {
		t.Fatalf("缺少二次确认应 400，实际 %d", got.Code)
	}
	ok := performJSONWithBearer(t, r, http.MethodDelete, "/api/v1/admin/entitlements/1",
		`{"reason":"违规获取，撤销免费学习","confirm":true}`, token)
	if ok.Code != http.StatusOK || !containsAll(ok.Body.String(), `"status":"revoked"`) {
		t.Fatalf("撤销权益失败: %d %s", ok.Code, ok.Body.String())
	}
	var revokedAt, reason string
	if err := database.QueryRow(`SELECT revoked_at, revoke_reason FROM entitlement WHERE id=1`).Scan(&revokedAt, &reason); err != nil {
		t.Fatal(err)
	}
	if revokedAt == "" || reason != "admin" {
		t.Fatalf("权益撤销字段异常: %s / %s", revokedAt, reason)
	}
	// 撤销授权 ≠ 退款：已发证书必须保留（PRD §7.3 G-7）。
	var certStatus string
	if err := database.QueryRow(`SELECT status FROM certificate WHERE cert_no='CW-CN-A202600001'`).Scan(&certStatus); err != nil || certStatus != "valid" {
		t.Fatalf("撤销权益不得撤证: %s %v", certStatus, err)
	}
	var audits int
	if err := database.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action='entitlement.revoke'`).Scan(&audits); err != nil || audits != 1 {
		t.Fatalf("撤销权益必须写审计: %d %v", audits, err)
	}
	if again := performJSONWithBearer(t, r, http.MethodDelete, "/api/v1/admin/entitlements/1",
		`{"reason":"重复","confirm":true}`, token); again.Code != http.StatusConflict {
		t.Fatalf("重复撤销应 409，实际 %d", again.Code)
	}
	revoked := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/entitlements?status=revoked", "", token)
	if revoked.Code != http.StatusOK || !containsAll(revoked.Body.String(), `"status":"revoked"`, `"revoke_reason":"admin"`) {
		t.Fatalf("撤销后台账异常: %d %s", revoked.Code, revoked.Body.String())
	}
}
