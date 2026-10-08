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

func TestAdminGrantAndBroadcast(t *testing.T) {
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
	if _, err = database.Exec(`INSERT INTO student(id, student_no, name, phone, id_card_hash, id_card_cipher, agreement_versions) VALUES(1,'S1','学员','13812345678',X'01',X'02','v1')`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(`INSERT INTO staff(id, account, name, role, password_hash, created_source) VALUES(2,'t','教师','teacher',X'01','seed')`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(`INSERT INTO major(id, name) VALUES(1,'样例专业')`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(`INSERT INTO course(id, course_code, source, name, line, major_id, status, teacher_id) VALUES(1,'C1','platform','课程','main',1,'published',2)`); err != nil {
		t.Fatal(err)
	}
	r := NewWithDatabase(testConfig(t, true), database).Router()

	login := performJSON(t, r, http.MethodPost, "/api/v1/admin/login", `{"account":"admin","password":"Temp@123"}`)
	token := extractJSONField(t, login.Body.String(), "token")

	// 授权免费学习
	grant := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/entitlements", `{"student_id":1,"course_id":1,"grant_reason":"校长特批"}`, token)
	if grant.Code != http.StatusOK {
		t.Fatalf("授权 = %d %s", grant.Code, grant.Body.String())
	}
	// 重复授权 → 409
	dup := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/entitlements", `{"student_id":1,"course_id":1,"grant_reason":"再次"}`, token)
	if dup.Code != http.StatusConflict {
		t.Fatalf("重复授权应 409，实际 %d", dup.Code)
	}
	// 缺事由 → 400
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/entitlements", `{"student_id":1,"course_id":1}`, token); got.Code != http.StatusBadRequest {
		t.Fatalf("缺事由应 400，实际 %d", got.Code)
	}
	// course_code 不存在 → 400
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/entitlements", `{"student_id":1,"course_code":"NOPE","grant_reason":"x"}`, token); got.Code != http.StatusBadRequest {
		t.Fatalf("course_code 不存在应 400，实际 %d", got.Code)
	}
	// course_code 解析成功但重复 → 409（覆盖解析成功分支）
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/entitlements", `{"student_id":1,"course_code":"C1","grant_reason":"x"}`, token); got.Code != http.StatusConflict {
		t.Fatalf("course_code 重复授权应 409，实际 %d", got.Code)
	}

	// 广播草稿 + 二次确认发送
	bc := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/broadcasts", `{"target":"all","title":"公告","body":"正文","idempotency_key":"k1"}`, token)
	if bc.Code != http.StatusOK || !containsAll(bc.Body.String(), `"broadcast_id"`) {
		t.Fatalf("广播草稿 = %d %s", bc.Code, bc.Body.String())
	}
	var bcResp struct {
		Data struct {
			BroadcastID int64 `json:"broadcast_id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(bc.Body.Bytes(), &bcResp)
	confirm := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/broadcasts/"+strconv.FormatInt(bcResp.Data.BroadcastID, 10)+"/confirm", `{}`, token)
	if confirm.Code != http.StatusOK {
		t.Fatalf("广播确认 = %d %s", confirm.Code, confirm.Body.String())
	}

	// 错误路径：撤销无权益 → 409；审核/上架不存在的课程 → 409；广播非法 target → 400
	if got := performJSONWithBearer(t, r, http.MethodDelete, "/api/v1/admin/entitlements/999/999", "", token); got.Code != http.StatusConflict {
		t.Fatalf("撤销无权益应 409，实际 %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/courses/NOPE/review", `{"verdict":"rejected","issue_list":"[]"}`, token); got.Code != http.StatusConflict {
		t.Fatalf("审核不存在课程应 409，实际 %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/courses/NOPE/publish", `{}`, token); got.Code != http.StatusConflict {
		t.Fatalf("上架不存在课程应 409，实际 %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/broadcasts", `{"target":"bad","title":"x","body":"y","idempotency_key":"k2"}`, token); got.Code != http.StatusBadRequest {
		t.Fatalf("广播非法 target 应 400，实际 %d", got.Code)
	}
}
