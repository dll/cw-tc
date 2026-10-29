package api

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/cw-tc/server/internal/identity"
	"github.com/cw-tc/server/internal/platform/db"
	"github.com/cw-tc/server/internal/staff"
	"github.com/cw-tc/server/migrations"
)

func TestAdminResetStudentPasswordFlow(t *testing.T) {
	ctx := context.Background()
	database, err := db.Open(ctx, db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := db.ApplyMigrations(ctx, database, migrations.FS, db.DialectSQLite); err != nil {
		t.Fatal(err)
	}
	if _, err := staff.Create(ctx, database, "admin", "管理员", staff.RoleAdminOps, "Temp@123", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := staff.Create(ctx, database, "teacher", "教师", staff.RoleTeacher, "Temp@123", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO student(id, student_no, name, phone, id_card_hash, id_card_cipher, agreement_versions, registration_review_status) VALUES(1,'S1','学员','13812345678',X'01',X'02','v1','approved')`); err != nil {
		t.Fatal(err)
	}
	cfg := testConfig(t, true)
	identityService := identity.NewService(database, []byte(cfg.IDCardEncKey), []byte(cfg.IDCardHMACKey))
	oldSession, err := identityService.CreateSession(ctx, 1, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}

	r := NewWithIdentityAndDatabase(cfg, identityService, database).Router()
	adminToken := extractJSONField(t, performJSON(t, r, http.MethodPost, "/api/v1/admin/login", `{"account":"admin","password":"Temp@123"}`).Body.String(), "token")
	teacherToken := extractJSONField(t, performJSON(t, r, http.MethodPost, "/api/v1/admin/login", `{"account":"teacher","password":"Temp@123"}`).Body.String(), "token")

	// 教师无重置权限
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/users/1/reset-password", `{"reason":"忘记密码","confirm":true}`, teacherToken); got.Code != http.StatusForbidden {
		t.Fatalf("教师重置密码必须 403: %d %s", got.Code, got.Body.String())
	}
	// 事由必填
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/users/1/reset-password", `{"confirm":true}`, adminToken); got.Code != http.StatusBadRequest {
		t.Fatalf("缺少事由应 400: %d %s", got.Code, got.Body.String())
	}
	// 生成重置码
	reset := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/users/1/reset-password", `{"reason":"学员忘记密码","confirm":true}`, adminToken)
	if reset.Code != http.StatusOK {
		t.Fatalf("管理员重置失败: %d %s", reset.Code, reset.Body.String())
	}
	code := extractJSONField(t, reset.Body.String(), "reset_code")
	if len(code) != 8 || !containsAll(reset.Body.String(), `"phone_last4":"5678"`) {
		t.Fatalf("重置码响应异常: %s", reset.Body.String())
	}
	var audits int
	if err := database.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action='user.reset_password'`).Scan(&audits); err != nil || audits != 1 {
		t.Fatalf("重置密码应写审计: %d %v", audits, err)
	}
	// 错误重置码
	if got := performJSON(t, r, http.MethodPost, "/api/v1/auth/password/reset", `{"phone":"13812345678","code":"00000000","new_password":"new-pass-1"}`); got.Code != http.StatusUnauthorized {
		t.Fatalf("错误重置码应 401: %d %s", got.Code, got.Body.String())
	}
	// 正确重置码
	ok := performJSON(t, r, http.MethodPost, "/api/v1/auth/password/reset", `{"phone":"13812345678","code":"`+code+`","new_password":"new-pass-1"}`)
	if ok.Code != http.StatusOK {
		t.Fatalf("重置密码失败: %d %s", ok.Code, ok.Body.String())
	}
	// 旧会话失效
	if got := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/me", "", oldSession.Token); got.Code != http.StatusUnauthorized {
		t.Fatalf("重置后旧会话必须失效: %d", got.Code)
	}
	// 新密码可登录
	login := performJSON(t, r, http.MethodPost, "/api/v1/auth/login", `{"phone":"13812345678","password":"new-pass-1"}`)
	if login.Code != http.StatusOK {
		t.Fatalf("新密码登录失败: %d %s", login.Code, login.Body.String())
	}
	studentToken := extractJSONField(t, login.Body.String(), "token")
	// 设备列表与退出全部
	devices := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/me/devices", "", studentToken)
	if devices.Code != http.StatusOK || !containsAll(devices.Body.String(), `"items"`, `"current":true`) {
		t.Fatalf("设备列表异常: %d %s", devices.Code, devices.Body.String())
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/me/logout-all", "", studentToken); got.Code != http.StatusOK {
		t.Fatalf("退出全部设备失败: %d %s", got.Code, got.Body.String())
	}
	if got := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/me", "", studentToken); got.Code != http.StatusUnauthorized {
		t.Fatalf("退出全部后会话应失效: %d", got.Code)
	}
	// 不存在的学员 / 停用账号
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/users/999/reset-password", `{"reason":"x","confirm":true}`, adminToken); got.Code != http.StatusNotFound {
		t.Fatalf("不存在学员应 404: %d %s", got.Code, got.Body.String())
	}
	if _, err := database.Exec(`UPDATE student SET status='suspended' WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/users/1/reset-password", `{"reason":"x","confirm":true}`, adminToken); got.Code != http.StatusConflict {
		t.Fatalf("停用账号应 409: %d %s", got.Code, got.Body.String())
	}
}
