package api

import (
	"net/http"
	"strings"
	"testing"
)

func TestAdminExportStudentDataLedger(t *testing.T) {
	server, database, superToken, _ := adminDetailServer(t)
	defer database.Close()
	r := server.Router()
	opsLogin := performJSON(t, r, http.MethodPost, "/api/v1/admin/login", `{"account":"ops","password":"Temp@123"}`)
	opsToken := extractJSONField(t, opsLogin.Body.String(), "token")

	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/exports",
		`{"student_id":1,"scope":"all","reason":"客服工单","confirm":true}`, opsToken); got.Code != http.StatusForbidden {
		t.Fatalf("运营管理员不得代导出，实际 %d", got.Code)
	}
	for _, body := range []string{
		`{"student_id":1,"scope":"all","reason":"","confirm":true}`,
		`{"student_id":1,"scope":"bad","reason":"工单","confirm":true}`,
		`{"student_id":1,"scope":"all","format":"md","reason":"工单","confirm":true}`,
		`{"student_id":0,"scope":"all","reason":"工单","confirm":true}`,
		`{"student_id":1,"scope":"all","reason":"工单","confirm":false}`,
	} {
		if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/exports", body, superToken); got.Code != http.StatusBadRequest {
			t.Fatalf("非法代导出应 400: %s -> %d", body, got.Code)
		}
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/exports",
		`{"student_id":999,"scope":"all","reason":"工单","confirm":true}`, superToken); got.Code != http.StatusNotFound {
		t.Fatalf("不存在学员应 404，实际 %d", got.Code)
	}
	ok := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/exports",
		`{"student_id":1,"scope":"learning","reason":"学员申请导出学习记录","confirm":true}`, superToken)
	if ok.Code != http.StatusOK || !containsAll(ok.Body.String(), `"scope":"learning"`, `"learning"`, `"notes"`) {
		t.Fatalf("代导出失败: %d %s", ok.Code, ok.Body.String())
	}
	if strings.Contains(ok.Body.String(), "13812345678") || strings.Contains(ok.Body.String(), "110101199001011237") {
		t.Fatalf("导出内容不得含明文手机号 / 身份证号: %s", ok.Body.String())
	}
	if strings.Contains(ok.Body.String(), `"orders"`) {
		t.Fatalf("learning 范围不应带订单: %s", ok.Body.String())
	}
	var initiator, reason, scope, operatorID string
	if err := database.QueryRow(`SELECT initiator, reason, scope, operator_id FROM data_export ORDER BY id DESC LIMIT 1`).
		Scan(&initiator, &reason, &scope, &operatorID); err != nil {
		t.Fatal(err)
	}
	if initiator != "admin" || reason == "" || scope != "learning" || operatorID == "" {
		t.Fatalf("导出留痕异常: %s %s %s %s", initiator, reason, scope, operatorID)
	}
	var audits int
	if err := database.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action='student.export_admin'`).Scan(&audits); err != nil || audits != 1 {
		t.Fatalf("代导出必须写审计: %d %v", audits, err)
	}
	ledger := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/exports", "", superToken)
	if ledger.Code != http.StatusOK || !containsAll(ledger.Body.String(), `"initiator":"admin"`, `"scope":"learning"`) {
		t.Fatalf("导出台账异常: %d %s", ledger.Code, ledger.Body.String())
	}
	if got := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/exports", "", opsToken); got.Code != http.StatusForbidden {
		t.Fatalf("运营管理员不得查看导出台账，实际 %d", got.Code)
	}
}

func TestSelfExportRecordsLedger(t *testing.T) {
	server, database, _, _ := adminDetailServer(t)
	defer database.Close()
	r := server.Router()
	session, err := server.identity.CreateSession(t.Context(), 1, timeNowUTC())
	if err != nil {
		t.Fatal(err)
	}
	got := performJSONWithCookie(t, r, http.MethodGet, "/api/v1/me/export", "", session.Token)
	if got.Code != http.StatusOK || !containsAll(got.Body.String(), `"learning"`, `"notes"`) {
		t.Fatalf("自助导出失败: %d %s", got.Code, got.Body.String())
	}
	var n int
	if err := database.QueryRow(`SELECT COUNT(*) FROM data_export WHERE initiator='self' AND subject_id=1`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("自助导出也必须留痕: %d %v", n, err)
	}
}
