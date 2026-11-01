package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
)

func TestAdminAccountHierarchyAndGuards(t *testing.T) {
	server, database, superToken, _ := adminDetailServer(t)
	defer database.Close()
	r := server.Router()
	opsLogin := performJSON(t, r, http.MethodPost, "/api/v1/admin/login", `{"account":"ops","password":"Temp@123"}`)
	opsToken := extractJSONField(t, opsLogin.Body.String(), "token")

	// 管理员列表 / 创建仅 super。
	if got := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/admins", "", opsToken); got.Code != http.StatusForbidden {
		t.Fatalf("ops 查看管理员列表应 403，实际 %d", got.Code)
	}
	list := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/admins", "", superToken)
	if list.Code != http.StatusOK || !containsAll(list.Body.String(), `"role":"admin_super"`, `"role":"admin_ops"`) {
		t.Fatalf("管理员列表异常: %d %s", list.Code, list.Body.String())
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/admins",
		`{"account":"ops2","name":"运营二","role":"admin_ops","password":"Temp@123"}`, opsToken); got.Code != http.StatusForbidden {
		t.Fatalf("ops 创建管理员应 403，实际 %d", got.Code)
	}
	created := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/admins",
		`{"account":"ops2","name":"运营二","role":"admin_ops","password":"Temp@123"}`, superToken)
	if created.Code != http.StatusOK || !containsAll(created.Body.String(), `"id"`) {
		t.Fatalf("super 创建管理员失败: %d %s", created.Code, created.Body.String())
	}
	createdID := jsonID(t, created.Body.String())

	// ops 不能通过 /staff 建管理员，但仍可开教师。
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/staff",
		`{"account":"ops3","name":"运营三","role":"admin_ops","password":"Temp@123"}`, opsToken); got.Code != http.StatusForbidden {
		t.Fatalf("ops 越权创建管理员应 403，实际 %d", got.Code)
	}
	teacher := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/staff",
		`{"account":"t2","name":"教师二","role":"teacher","password":"Temp@123"}`, opsToken)
	if teacher.Code != http.StatusOK {
		t.Fatalf("ops 开通教师应允许: %d %s", teacher.Code, teacher.Body.String())
	}
	teacherID := jsonID(t, teacher.Body.String())

	// 停用守卫：不能停自己；ops 不能停管理员；最后一个超管不能停。
	if got := performJSONWithBearer(t, r, http.MethodDelete, "/api/v1/admin/admins/1", "", superToken); got.Code != http.StatusConflict {
		t.Fatalf("不能停用自己，实际 %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodDelete, "/api/v1/admin/admins/1", "", opsToken); got.Code != http.StatusForbidden {
		t.Fatalf("ops 停用超管应 403，实际 %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/staff/1/disable", `{}`, opsToken); got.Code != http.StatusForbidden {
		t.Fatalf("ops 通过 staff 停用超管应 403，实际 %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodDelete, "/api/v1/admin/admins/"+createdID, "", superToken); got.Code != http.StatusOK {
		t.Fatalf("停用管理员失败: %d %s", got.Code, got.Body.String())
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/staff/"+teacherID+"/disable", `{}`, opsToken); got.Code != http.StatusOK {
		t.Fatalf("ops 停用教师应允许: %d %s", got.Code, got.Body.String())
	}
	// 超管互相停用（非最后一个）允许。
	second := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/admins",
		`{"account":"boss2","name":"超管二","role":"admin_super","password":"Temp@123"}`, superToken)
	if second.Code != http.StatusOK {
		t.Fatalf("创建第二个超管失败: %d %s", second.Code, second.Body.String())
	}
	secondID, _ := strconv.ParseInt(jsonID(t, second.Body.String()), 10, 64)
	if got := performJSONWithBearer(t, r, http.MethodDelete, "/api/v1/admin/admins/"+strconv.FormatInt(secondID, 10), "", superToken); got.Code != http.StatusOK {
		t.Fatalf("停用非最后超管应允许: %d", got.Code)
	}
	var audits int
	if err := database.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action IN ('admin.create','admin.disable','staff.create','staff.disable')`).Scan(&audits); err != nil || audits < 4 {
		t.Fatalf("账号管理必须写审计: %d %v", audits, err)
	}
}

func jsonID(t *testing.T, raw string) string {
	t.Helper()
	var payload struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil || payload.Data.ID <= 0 {
		t.Fatalf("响应缺少 id: %s", raw)
	}
	return strconv.FormatInt(payload.Data.ID, 10)
}
