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

func TestAdminCourseManagementListUnpublishAndRepublish(t *testing.T) {
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
	if _, err := database.Exec(`INSERT INTO course(course_code,source,name,intro,line,lecture_count,major_id,status,teacher_id,price_cents,cert_names,cert_pass_lines,content_version,published_version) VALUES
		('PUB','platform','已上架课程','这是课程简介，用于列表展示','main',12,1,'published',2,5000,'{"L1":"证书"}','{"L1":60}',2,1),
		('DRAFT','platform','建设中课程','建设中简介','main',3,1,'building',2,0,NULL,NULL,1,NULL)`); err != nil {
		t.Fatal(err)
	}
	var pubID int64
	if err := database.QueryRow(`SELECT id FROM course WHERE course_code='PUB'`).Scan(&pubID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO lecture(course_id,no,title) VALUES($1,1,'第一讲')`, pubID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO student(id,student_no,name,phone,id_card_hash,id_card_cipher,agreement_versions,registration_review_status) VALUES
		(1,'S1','学员一','13812345678',X'01',X'02','v1','approved'),
		(2,'S2','学员二','13812345679',X'03',X'04','v1','approved')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO entitlement(student_id,course_id,source,granted_at) VALUES(1,$1,'free','2026-01-01T00:00:00Z')`, pubID); err != nil {
		t.Fatal(err)
	}
	cfg := testConfig(t, true)
	identityService := identity.NewService(database, []byte(cfg.IDCardEncKey), []byte(cfg.IDCardHMACKey))
	entitledSession, _ := identityService.CreateSession(ctx, 1, time.Now().UTC())
	otherSession, _ := identityService.CreateSession(ctx, 2, time.Now().UTC())
	r := NewWithIdentityAndDatabase(cfg, identityService, database).Router()
	adminToken := extractJSONField(t, performJSON(t, r, http.MethodPost, "/api/v1/admin/login", `{"account":"admin","password":"Temp@123"}`).Body.String(), "token")
	teacherToken := extractJSONField(t, performJSON(t, r, http.MethodPost, "/api/v1/admin/login", `{"account":"teacher","password":"Temp@123"}`).Body.String(), "token")

	list := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/courses", "", adminToken)
	if list.Code != http.StatusOK || !containsAll(list.Body.String(),
		`"intro":"这是课程简介，用于列表展示"`, `"lecture_count":12`,
		`"content_version":2`, `"published_version":1`, `"has_unpublished_changes":true`,
		`"stats":{"building":1,"published":1}`, `"total":2`, `"total_lectures":15`) {
		t.Fatalf("课程列表统计/简介/讲次/版本异常: %d %s", list.Code, list.Body.String())
	}
	filtered := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/courses?status=building&q=建设中", "", adminToken)
	if filtered.Code != http.StatusOK || !containsAll(filtered.Body.String(), `"course_code":"DRAFT"`) || containsAll(filtered.Body.String(), `"course_code":"PUB"`) {
		t.Fatalf("课程筛选异常: %d %s", filtered.Code, filtered.Body.String())
	}
	if got := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/courses?source=platform", "", adminToken); got.Code != http.StatusOK || !containsAll(got.Body.String(), `"total":2`) {
		t.Fatalf("来源筛选异常: %d %s", got.Code, got.Body.String())
	}

	// 教师不能下架/上架
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/courses/PUB/unpublish", `{"reason":"测试","confirm":true}`, teacherToken); got.Code != http.StatusForbidden {
		t.Fatalf("教师下架必须 403: %d %s", got.Code, got.Body.String())
	}
	// 下架必须带事由
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/courses/PUB/unpublish", `{"confirm":true}`, adminToken); got.Code != http.StatusBadRequest {
		t.Fatalf("缺事由应 400: %d %s", got.Code, got.Body.String())
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/courses/PUB/unpublish", `{"reason":"内容整改","confirm":true}`, adminToken); got.Code != http.StatusOK {
		t.Fatalf("下架失败: %d %s", got.Code, got.Body.String())
	}
	var status string
	_ = database.QueryRow(`SELECT status FROM course WHERE course_code='PUB'`).Scan(&status)
	if status != "unpublished" {
		t.Fatalf("下架后状态=%s", status)
	}
	var audits int
	_ = database.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action='course.unpublish'`).Scan(&audits)
	if audits != 1 {
		t.Fatalf("下架应写审计 1 条，实际 %d", audits)
	}
	// 公网列表不可见
	public := performJSON(t, r, http.MethodGet, "/api/v1/courses", "")
	if containsAll(public.Body.String(), `"course_code":"PUB"`) {
		t.Fatalf("下架课程不应出现在课程广场: %s", public.Body.String())
	}
	// 已购学员仍可访问；未购学员 403
	if got := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/courses/PUB/lectures/1/resources", "", entitledSession.Token); got.Code != http.StatusOK {
		t.Fatalf("已购学员访问下架课程资源应 200: %d %s", got.Code, got.Body.String())
	}
	if got := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/courses/PUB/lectures/1/resources", "", otherSession.Token); got.Code != http.StatusForbidden {
		t.Fatalf("未购学员访问下架课程资源应 403: %d %s", got.Code, got.Body.String())
	}
	if got := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/courses/PUB/lectures/1/practice", "", entitledSession.Token); got.Code != http.StatusOK {
		t.Fatalf("已购学员访问下架课程练习应 200: %d %s", got.Code, got.Body.String())
	}
	if got := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/courses/PUB/lectures/1/practice", "", otherSession.Token); got.Code != http.StatusForbidden {
		t.Fatalf("未购学员访问下架课程练习应 403: %d %s", got.Code, got.Body.String())
	}
	// 重新上架
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/courses/PUB/publish", "", adminToken); got.Code != http.StatusOK {
		t.Fatalf("重新上架失败: %d %s", got.Code, got.Body.String())
	}
	_ = database.QueryRow(`SELECT status FROM course WHERE course_code='PUB'`).Scan(&status)
	if status != "published" {
		t.Fatalf("重新上架后状态=%s", status)
	}
	_ = database.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action='course.publish'`).Scan(&audits)
	if audits != 1 {
		t.Fatalf("重新上架应写审计 1 条，实际 %d", audits)
	}
	var publishedVersion, revisionCount int
	_ = database.QueryRow(`SELECT published_version FROM course WHERE course_code='PUB'`).Scan(&publishedVersion)
	if publishedVersion != 2 {
		t.Fatalf("重新上架后 published_version=%d，期望 2", publishedVersion)
	}
	_ = database.QueryRow(`SELECT COUNT(*) FROM course_revision WHERE course_code='PUB'`).Scan(&revisionCount)
	if revisionCount != 1 {
		t.Fatalf("重新上架应新增 1 条版本记录，实际 %d", revisionCount)
	}
	rev := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/courses/PUB/revisions", "", adminToken)
	if rev.Code != http.StatusOK || !containsAll(rev.Body.String(), `"revision_no":1`, `"content_version":2`) {
		t.Fatalf("版本历史异常: %d %s", rev.Code, rev.Body.String())
	}
	// 专业与免费讲次（管理员 + 审计）
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/courses/PUB/major", `{"major_id":1}`, adminToken); got.Code != http.StatusOK {
		t.Fatalf("设置专业失败: %d %s", got.Code, got.Body.String())
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/courses/PUB/major", `{"major_id":999}`, adminToken); got.Code != http.StatusNotFound {
		t.Fatalf("不存在的专业应 404: %d %s", got.Code, got.Body.String())
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/courses/PUB/major", `{"major_id":1}`, teacherToken); got.Code != http.StatusForbidden {
		t.Fatalf("教师设置专业应 403: %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/courses/PUB/free-preview", `{"count":2}`, adminToken); got.Code != http.StatusOK {
		t.Fatalf("设置免费讲次失败: %d %s", got.Code, got.Body.String())
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/courses/PUB/free-preview", `{"count":-1}`, adminToken); got.Code != http.StatusBadRequest {
		t.Fatalf("负数免费讲次应 400: %d", got.Code)
	}
	_ = database.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action='course.major_change'`).Scan(&audits)
	if audits != 1 {
		t.Fatalf("专业变更应写审计 1 条，实际 %d", audits)
	}
	_ = database.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action='course.free_preview_change'`).Scan(&audits)
	if audits != 1 {
		t.Fatalf("免费讲次变更应写审计 1 条，实际 %d", audits)
	}
	// building 不能直接上架 / 下架
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/courses/DRAFT/publish", "", adminToken); got.Code != http.StatusConflict {
		t.Fatalf("building 直接上架应 409: %d %s", got.Code, got.Body.String())
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/courses/DRAFT/unpublish", `{"reason":"x","confirm":true}`, adminToken); got.Code != http.StatusConflict {
		t.Fatalf("building 下架应 409: %d %s", got.Code, got.Body.String())
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/courses/PUB/unpublish", `{"reason":"x","confirm":false}`, adminToken); got.Code != http.StatusBadRequest {
		t.Fatalf("未二次确认下架应 400: %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/courses/NOPE/publish", "", adminToken); got.Code != http.StatusConflict {
		t.Fatalf("不存在课程上架应 409: %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/courses/NOPE/free-preview", `{"count":1}`, adminToken); got.Code != http.StatusNotFound {
		t.Fatalf("不存在课程设置免费讲次应 404: %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/courses/PUB/free-preview", `{`, adminToken); got.Code != http.StatusBadRequest {
		t.Fatalf("非法 JSON 应 400: %d", got.Code)
	}
}
