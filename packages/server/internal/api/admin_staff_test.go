package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/cw-tc/server/internal/platform/db"
	"github.com/cw-tc/server/internal/staff"
	"github.com/cw-tc/server/migrations"
)

func TestAdminStaffAndRefundRoutes(t *testing.T) {
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
	if _, err = database.Exec(`INSERT INTO major(id, name) VALUES(1,'样例专业')`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(`INSERT INTO staff(id, account, name, role, password_hash, created_source) VALUES(99,'t','教师','teacher',X'01','seed')`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(`INSERT INTO course(id, course_code, source, name, line, major_id, status, teacher_id, price_cents) VALUES(1,'C1','platform','课程','main',1,'published',99,1000)`); err != nil {
		t.Fatal(err)
	}
	r := NewWithDatabase(testConfig(t, true), database).Router()

	login := performJSON(t, r, http.MethodPost, "/api/v1/admin/login", `{"account":"admin","password":"Temp@123"}`)
	token := extractJSONField(t, login.Body.String(), "token")

	// 开通教师
	create := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/staff", `{"account":"teacher1","name":"张老师","role":"teacher","password":"Temp@123"}`, token)
	if create.Code != http.StatusOK || !containsAll(create.Body.String(), `"id"`) {
		t.Fatalf("开通教师 = %d %s", create.Code, create.Body.String())
	}
	var createResp struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(create.Body.Bytes(), &createResp)
	tid := createResp.Data.ID

	// admin_ops 建 super → 403
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/staff", `{"account":"s2","name":"超管","role":"admin_super","password":"Temp@123"}`, token); got.Code != http.StatusForbidden {
		t.Fatalf("admin_ops 建 super 应 403，实际 %d", got.Code)
	}
	// 停用（无课程）
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/staff/"+strconv.FormatInt(tid, 10)+"/disable", `{}`, token); got.Code != http.StatusOK {
		t.Fatalf("停用 = %d %s", got.Code, got.Body.String())
	}
	// 退款未二次确认 → 400
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/orders/O1/refund", `{"kind":"normal","amount_cents":100,"reason":"退款"}`, token); got.Code != http.StatusBadRequest {
		t.Fatalf("退款未确认应 400，实际 %d", got.Code)
	}
	// 错误路径：重复账号 / 停用不存在 / 移交不存在课程 / 退款不存在订单
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/staff", `{"account":"admin","name":"重复","role":"teacher","password":"Temp@123"}`, token); got.Code != http.StatusConflict {
		t.Fatalf("重复账号应 409，实际 %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/staff/999/disable", `{}`, token); got.Code != http.StatusConflict {
		t.Fatalf("停用不存在账号应 409，实际 %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/courses/transfer", `{"course_id":999,"to_teacher_id":1}`, token); got.Code != http.StatusConflict {
		t.Fatalf("移交不存在课程应 409，实际 %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/orders/NOPE/refund", `{"kind":"normal","amount_cents":100,"reason":"退款","confirm":true}`, token); got.Code != http.StatusConflict {
		t.Fatalf("退款不存在订单应 409，实际 %d", got.Code)
	}
	// 改价：未二次确认 → 400；确认 → 200 且写审计
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/courses/C1/price", `{"price_cents":2000}`, token); got.Code != http.StatusBadRequest {
		t.Fatalf("改价未确认应 400，实际 %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/courses/C1/price", `{"price_cents":2000,"confirm":true}`, token); got.Code != http.StatusOK {
		t.Fatalf("改价应 200 = %d %s", got.Code, got.Body.String())
	}
	var n int
	_ = database.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action='course.price_change'`).Scan(&n)
	if n != 1 {
		t.Fatalf("改价应写审计 1 条，实际 %d", n)
	}
}
