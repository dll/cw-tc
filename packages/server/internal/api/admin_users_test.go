package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/cw-tc/server/internal/platform/db"
	"github.com/cw-tc/server/internal/staff"
	"github.com/cw-tc/server/migrations"
)

func TestAdminListUsersAndReveal(t *testing.T) {
	database, err := db.Open(context.Background(), db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := db.ApplyMigrations(context.Background(), database, migrations.FS, db.DialectSQLite); err != nil {
		t.Fatal(err)
	}
	if _, err := staff.Create(context.Background(), database, "admin", "管理员", staff.RoleAdminOps, "Temp@123", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := staff.Create(context.Background(), database, "teacher1", "教师", staff.RoleTeacher, "Temp@123", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := staff.Create(context.Background(), database, "super1", "超管", staff.RoleAdminSuper, "Temp@123", 0); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(`INSERT INTO student(id, student_no, name, phone, id_card_hash, id_card_cipher, agreement_versions) VALUES(1,'S1','学员','13812345678',X'01',X'02','v1')`); err != nil {
		t.Fatal(err)
	}
	r := NewWithDatabase(testConfig(t, true), database).Router()

	login := performJSON(t, r, http.MethodPost, "/api/v1/admin/login", `{"account":"admin","password":"Temp@123"}`)
	token := extractJSONField(t, login.Body.String(), "token")

	// 列表脱敏
	list := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/users", "", token)
	if list.Code != http.StatusOK || !containsAll(list.Body.String(), `"phone":"138****5678"`) {
		t.Fatalf("用户列表应脱敏 = %d %s", list.Code, list.Body.String())
	}
	for _, tc := range []struct{ query, wantType, wantAccount string }{
		{"student", "student", ""},
		{"teacher", "teacher", "teacher1"},
		{"admin_super", "admin_super", "super1"},
		{"admin_ops", "admin_ops", "admin"},
	} {
		got := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/users?type="+tc.query, "", token)
		var body struct {
			Data struct {
				Items []struct {
					UserType string `json:"user_type"`
					Account  string `json:"account"`
					Phone    string `json:"phone"`
				} `json:"items"`
				Total int `json:"total"`
			} `json:"data"`
		}
		if err := json.Unmarshal(got.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if got.Code != http.StatusOK || body.Data.Total != 1 || len(body.Data.Items) != 1 ||
			body.Data.Items[0].UserType != tc.wantType || body.Data.Items[0].Account != tc.wantAccount {
			t.Fatalf("按 %s 筛选错误: %d %s", tc.query, got.Code, got.Body.String())
		}
		if tc.query != "student" && body.Data.Items[0].Phone != "" {
			t.Fatalf("内部人员手机号不得返回: %s", got.Body.String())
		}
	}
	if got := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/users?limit=1&offset=1", "", token); got.Code != http.StatusOK || !containsAll(got.Body.String(), `"total":4`) {
		t.Fatalf("统一列表分页或总数错误: %d %s", got.Code, got.Body.String())
	}
	if got := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/users?type=unknown", "", token); got.Code != http.StatusBadRequest {
		t.Fatalf("非法用户类型应拒绝: %d", got.Code)
	}
	// 看全号需事由
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/users/1/reveal", `{}`, token); got.Code != http.StatusBadRequest {
		t.Fatalf("无事故应 400，实际 %d", got.Code)
	}
	// 看全号（事由）→ 返回全号 + 写审计
	reveal := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/users/1/reveal", `{"reason":"投诉处理"}`, token)
	if reveal.Code != http.StatusOK || !containsAll(reveal.Body.String(), `"phone":"13812345678"`) {
		t.Fatalf("看全号失败 = %d %s", reveal.Code, reveal.Body.String())
	}
	var n int
	_ = database.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action='user.reveal_phone'`).Scan(&n)
	if n != 1 {
		t.Fatalf("看全号应写审计 1 条，实际 %d", n)
	}
	// 审计日志列表
	if got := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/audit", "", token); got.Code != http.StatusOK || !containsAll(got.Body.String(), `"action":"user.reveal_phone"`) {
		t.Fatalf("审计日志列表 = %d %s", got.Code, got.Body.String())
	}
	if got := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/audit?action=user.reveal_phone", "", token); got.Code != http.StatusOK || !containsAll(got.Body.String(), `"items"`) {
		t.Fatalf("按动作过滤审计 = %d %s", got.Code, got.Body.String())
	}
	// 不存在的用户 → 404
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/users/999/reveal", `{"reason":"x"}`, token); got.Code != http.StatusNotFound {
		t.Fatalf("看全号不存在用户应 404，实际 %d", got.Code)
	}
}
