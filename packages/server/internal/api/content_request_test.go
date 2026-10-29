package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cw-tc/server/internal/identity"
	"github.com/cw-tc/server/internal/platform/db"
	"github.com/cw-tc/server/internal/staff"
	"github.com/cw-tc/server/migrations"
)

func TestContentRequestFlow(t *testing.T) {
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
	if _, err := database.Exec(`INSERT INTO course(course_code,source,name,line,major_id,status,teacher_id,cert_names,cert_pass_lines) VALUES
		('C1','platform','课程一','main',1,'published',2,'{"L1":"证书"}','{"L1":60}'),
		('C2','platform','课程二','main',1,'published',2,'{"L1":"证书"}','{"L1":60}'),
		('C3','platform','课程三','main',1,'published',2,'{"L1":"证书"}','{"L1":60}')`); err != nil {
		t.Fatal(err)
	}
	r := NewWithDatabase(testConfig(t, true), database).Router()
	adminToken := extractJSONField(t, performJSON(t, r, http.MethodPost, "/api/v1/admin/login", `{"account":"admin","password":"Temp@123"}`).Body.String(), "token")
	teacherToken := extractJSONField(t, performJSON(t, r, http.MethodPost, "/api/v1/admin/login", `{"account":"teacher","password":"Temp@123"}`).Body.String(), "token")

	create := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/teacher/courses/C1/requests",
		`{"kind":"offline","reason_kind":"content_error","reason_text":"发现答案错误"}`, teacherToken)
	if create.Code != http.StatusOK {
		t.Fatalf("创建下架申请失败: %d %s", create.Code, create.Body.String())
	}
	var createResp struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(create.Body.Bytes(), &createResp); err != nil || createResp.Data.ID <= 0 {
		t.Fatalf("申请 id 异常: %s", create.Body.String())
	}
	requestID := createResp.Data.ID
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/teacher/courses/C1/requests",
		`{"kind":"offline","reason_kind":"content_error"}`, teacherToken); got.Code != http.StatusConflict {
		t.Fatalf("重复申请应 409: %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/teacher/courses/C1/requests",
		`{"kind":"bad","reason_kind":"content_error"}`, teacherToken); got.Code != http.StatusBadRequest {
		t.Fatalf("非法类型应 400: %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/teacher/courses/C1/requests",
		`{"kind":"offline","reason_kind":"content_error"}`, adminToken); got.Code != http.StatusNotFound {
		t.Fatalf("非本人课程应 404: %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/teacher/requests", "", teacherToken); got.Code != http.StatusOK || !containsAll(got.Body.String(), `"course_code":"C1"`, `"pending"`) {
		t.Fatalf("教师申请列表异常: %d %s", got.Code, got.Body.String())
	}
	if got := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/requests?status=pending", "", adminToken); got.Code != http.StatusOK || !containsAll(got.Body.String(), `"id":`+strconv.FormatInt(requestID, 10)) {
		t.Fatalf("管理员待办异常: %d %s", got.Code, got.Body.String())
	}
	if got := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/requests", "", teacherToken); got.Code != http.StatusForbidden {
		t.Fatalf("教师读取管理员待办应 403: %d", got.Code)
	}
	approve := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/requests/"+strconv.FormatInt(requestID, 10)+"/approve", `{"confirm":true}`, adminToken)
	if approve.Code != http.StatusOK {
		t.Fatalf("批准申请失败: %d %s", approve.Code, approve.Body.String())
	}
	var courseStatus string
	_ = database.QueryRow(`SELECT status FROM course WHERE course_code='C1'`).Scan(&courseStatus)
	if courseStatus != "unpublished" {
		t.Fatalf("批准后课程应下架，实际 %s", courseStatus)
	}
	var audits int
	_ = database.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action='content_request.approve'`).Scan(&audits)
	if audits != 1 {
		t.Fatalf("批准应写审计 1 条，实际 %d", audits)
	}
	_ = database.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action='course.unpublish'`).Scan(&audits)
	if audits != 1 {
		t.Fatalf("批准下架应写课程审计 1 条，实际 %d", audits)
	}
	if got := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/teacher/requests", "", teacherToken); got.Code != http.StatusOK || !containsAll(got.Body.String(), `"status":"approved"`) {
		t.Fatalf("教师应看到申请已批准: %d %s", got.Code, got.Body.String())
	}

	// 紧急纠错：创建后暂停新测评
	urgent := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/teacher/courses/C2/requests",
		`{"kind":"urgent_fix","reason_kind":"content_error","reason_text":"答案严重错误"}`, teacherToken)
	if urgent.Code != http.StatusOK {
		t.Fatalf("紧急纠错申请失败: %d %s", urgent.Code, urgent.Body.String())
	}

	// 驳回流程
	rejectReq := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/teacher/courses/C3/requests",
		`{"kind":"offline","reason_kind":"other","reason_text":"个人原因"}`, teacherToken)
	var rejectResp struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rejectReq.Body.Bytes(), &rejectResp); err != nil || rejectResp.Data.ID <= 0 {
		t.Fatalf("驳回申请 id 异常: %s", rejectReq.Body.String())
	}
	rejectID := rejectResp.Data.ID
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/requests/"+strconv.FormatInt(rejectID, 10)+"/reject", `{"confirm":true}`, adminToken); got.Code != http.StatusBadRequest {
		t.Fatalf("驳回缺原因应 400: %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/requests/"+strconv.FormatInt(rejectID, 10)+"/reject", `{"reason":"内容已修改","confirm":true}`, adminToken); got.Code != http.StatusOK {
		t.Fatalf("驳回失败: %d %s", got.Code, got.Body.String())
	}
	var rejectStatus string
	_ = database.QueryRow(`SELECT status FROM content_request WHERE id=$1`, rejectID).Scan(&rejectStatus)
	if rejectStatus != "rejected" {
		t.Fatalf("驳回后状态=%s", rejectStatus)
	}
}

func TestUrgentFixBlocksExam(t *testing.T) {
	ctx := context.Background()
	database, err := db.Open(ctx, db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := db.ApplyMigrations(ctx, database, migrations.FS, db.DialectSQLite); err != nil {
		t.Fatal(err)
	}
	service := identity.NewService(database, []byte(strings.Repeat("a", 32)), []byte("hmac-key-for-cwTC-tests-012345678901234"))
	student, err := service.Register(ctx, identity.RegisterInput{Name: "样例学员", Phone: "13812345678", IDCard: "110101199001011237", Password: "password-1"})
	if err != nil {
		t.Fatal(err)
	}
	session, err := service.CreateSession(ctx, student.ID, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO staff(id, account, name, role, password_hash, created_source) VALUES(1,'teacher','教师','teacher',X'01','seed')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO major(id,name) VALUES(1,'样例专业')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO course(id,course_code,source,name,line,major_id,status,teacher_id,cert_pass_lines) VALUES(1,'C1','platform','课程','main',1,'published',1,'{"L1":1.5}')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO question(course_id,qno,level,type,gradability,stem,answer,score) VALUES(1,'L1-01-01','L1','single','auto','题干','A',1.5)`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO entitlement(student_id,course_id,source,granted_at) VALUES($1,1,'free','2026-01-01T00:00:00Z')`, student.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO content_request(course_code,teacher_id,kind,reason_kind,reason_text,status,created_at) VALUES('C1',1,'urgent_fix','content_error','答案错误','pending',$1)`, time.Now().UTC().Format(time.RFC3339)); err != nil {
		t.Fatal(err)
	}
	r := NewWithIdentityAndDatabase(testConfig(t, true), service, database).Router()
	start := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/exams", `{"course_code":"C1","level":"L1"}`, session.Token)
	if start.Code != http.StatusConflict || !strings.Contains(start.Body.String(), "核对中") {
		t.Fatalf("紧急纠错期间应拒绝开考: %d %s", start.Code, start.Body.String())
	}
}
