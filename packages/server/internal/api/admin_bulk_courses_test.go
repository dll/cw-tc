package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/cw-tc/server/internal/platform/db"
	"github.com/cw-tc/server/internal/staff"
	"github.com/cw-tc/server/migrations"
)

func TestAdminBulkCourseActions(t *testing.T) {
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
	if _, err := database.Exec(`INSERT INTO major(id,name) VALUES(1,'样例专业')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO course(course_code,source,name,line,lecture_count,major_id,status,teacher_id,price_cents,cert_names,cert_pass_lines) VALUES
		('B1','platform','批量上架一','main',2,1,'approved',2,5000,'{"L1":"证书"}','{"L1":60}'),
		('B2','platform','批量上架二','main',3,1,'approved',2,5000,'{"L1":"证书"}','{"L1":60}'),
		('B3','platform','草稿','main',1,1,'proposed',2,5000,'{"L1":"证书"}','{"L1":60}'),
		('U1','platform','批量下架一','main',2,1,'published',2,5000,'{"L1":"证书"}','{"L1":60}'),
		('U2','platform','批量下架二','main',2,1,'published',2,5000,'{"L1":"证书"}','{"L1":60}')`); err != nil {
		t.Fatal(err)
	}
	r := NewWithDatabase(testConfig(t, true), database).Router()
	adminToken := extractJSONField(t, performJSON(t, r, http.MethodPost, "/api/v1/admin/login", `{"account":"admin","password":"Temp@123"}`).Body.String(), "token")
	teacherToken := extractJSONField(t, performJSON(t, r, http.MethodPost, "/api/v1/admin/login", `{"account":"teacher","password":"Temp@123"}`).Body.String(), "token")

	if got := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/users", "", teacherToken); got.Code != http.StatusForbidden {
		t.Fatalf("教师读取用户列表必须 403: %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/users", "", adminToken); got.Code != http.StatusOK {
		t.Fatalf("管理员读取用户列表应 200: %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/bulk-courses/publish", `{"course_codes":["B1","B2","B3"],"confirm":true}`, teacherToken); got.Code != http.StatusForbidden {
		t.Fatalf("教师批量上架应 403: %d", got.Code)
	}
	publish := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/bulk-courses/publish", `{"course_codes":["B1","B2","B3"],"confirm":true}`, adminToken)
	if publish.Code != http.StatusOK || !containsAll(publish.Body.String(), `"published":2`, `"course_code":"B3"`) {
		t.Fatalf("批量上架异常: %d %s", publish.Code, publish.Body.String())
	}
	var status string
	_ = database.QueryRow(`SELECT status FROM course WHERE course_code='B1'`).Scan(&status)
	if status != "published" {
		t.Fatalf("B1 应已上架，实际 %s", status)
	}
	var revisions int
	_ = database.QueryRow(`SELECT COUNT(*) FROM course_revision WHERE course_code IN ('B1','B2')`).Scan(&revisions)
	if revisions != 2 {
		t.Fatalf("批量上架应新增 2 条版本记录，实际 %d", revisions)
	}
	var audits int
	_ = database.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action='course.publish'`).Scan(&audits)
	if audits != 2 {
		t.Fatalf("批量上架应写 2 条审计，实际 %d", audits)
	}

	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/bulk-courses/unpublish", `{"course_codes":["U1","U2"],"confirm":true}`, adminToken); got.Code != http.StatusBadRequest {
		t.Fatalf("批量下架缺事由应 400: %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/bulk-courses/unpublish", `{"course_codes":["U1","U2"],"reason":"内容整改","confirm":true}`, teacherToken); got.Code != http.StatusForbidden {
		t.Fatalf("教师批量下架应 403: %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/bulk-courses/unpublish", `{"course_codes":[],"reason":"x","confirm":true}`, adminToken); got.Code != http.StatusBadRequest {
		t.Fatalf("空列表批量下架应 400: %d", got.Code)
	}
	unpublish := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/bulk-courses/unpublish", `{"course_codes":["U1","U2"],"reason":"内容整改","confirm":true}`, adminToken)
	if unpublish.Code != http.StatusOK || !containsAll(unpublish.Body.String(), `"unpublished":2`) {
		t.Fatalf("批量下架异常: %d %s", unpublish.Code, unpublish.Body.String())
	}
	_ = database.QueryRow(`SELECT status FROM course WHERE course_code='U1'`).Scan(&status)
	if status != "unpublished" {
		t.Fatalf("U1 应已下架，实际 %s", status)
	}
	_ = database.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action='course.unpublish'`).Scan(&audits)
	if audits != 2 {
		t.Fatalf("批量下架应写 2 条审计，实际 %d", audits)
	}
}
