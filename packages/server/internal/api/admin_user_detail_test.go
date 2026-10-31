package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/cw-tc/server/internal/identity"
	"github.com/cw-tc/server/internal/platform/db"
	"github.com/cw-tc/server/internal/staff"
	"github.com/cw-tc/server/migrations"
)

func adminDetailServer(t *testing.T) (*Server, *sql.DB, string, int64) {
	t.Helper()
	database, err := db.Open(context.Background(), db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.ApplyMigrations(context.Background(), database, migrations.FS, db.DialectSQLite); err != nil {
		database.Close()
		t.Fatal(err)
	}
	service := identity.NewService(database,
		[]byte("01234567890123456789012345678901"),
		[]byte("hmac-key-for-cwTC-tests-012345678901234"))
	student, err := service.Register(context.Background(), identity.RegisterInput{
		Name: "样例学员", Phone: "13812345678", IDCard: "110101199001011237", Password: "password-1",
	})
	if err != nil {
		database.Close()
		t.Fatal(err)
	}
	bossID, err := staff.Create(context.Background(), database, "boss", "超级管理员", staff.RoleAdminSuper, "Temp@123", 0)
	if err != nil {
		database.Close()
		t.Fatal(err)
	}
	teacherID, err := staff.Create(context.Background(), database, "t1", "教师一", staff.RoleTeacher, "Temp@123", 0)
	if err != nil {
		database.Close()
		t.Fatal(err)
	}
	if _, err := staff.Create(context.Background(), database, "ops", "运营", staff.RoleAdminOps, "Temp@123", 0); err != nil {
		database.Close()
		t.Fatal(err)
	}
	stmts := []string{
		`INSERT INTO major(id,name) VALUES(1,'样例专业')`,
		`INSERT INTO course(id,course_code,source,name,line,major_id,status,teacher_id,price_cents) VALUES(1,'C1','platform','样例课程','main',1,'published',` + strconv.FormatInt(teacherID, 10) + `,1000)`,
		`INSERT INTO lecture(id,course_id,no,title) VALUES(1,1,1,'第一讲'),(2,1,2,'第二讲')`,
		`INSERT INTO entitlement(student_id,course_id,source,granted_at) VALUES(` + strconv.FormatInt(student.ID, 10) + `,1,'free','2026-01-01T00:00:00Z')`,
		`INSERT INTO lecture_progress(student_id,lecture_id,last_studied_at,completed_at) VALUES(` + strconv.FormatInt(student.ID, 10) + `,1,'2026-10-30T02:00:00Z','2026-10-30T02:00:00Z')`,
		`INSERT INTO lecture_progress(student_id,lecture_id,last_studied_at) VALUES(` + strconv.FormatInt(student.ID, 10) + `,2,'2026-10-31T02:00:00Z')`,
		`INSERT INTO exam_session(id,student_id,course_id,level,paper,status,server_deadline_at,created_at) VALUES(1,` + strconv.FormatInt(student.ID, 10) + `,1,'L1','{}','graded','2026-10-31T03:00:00Z','2026-10-31T02:30:00Z')`,
		`INSERT INTO exam_result(exam_session_id,student_id,course_id,level,total_score,band,is_pass,created_at) VALUES(1,` + strconv.FormatInt(student.ID, 10) + `,1,'L1',88,'good',1,'2026-10-31T02:45:00Z')`,
		`INSERT INTO certificate(cert_no,student_id,course_id,level,score,band,issued_at,expires_at) VALUES('CW-CN-A202600001',` + strconv.FormatInt(student.ID, 10) + `,1,'L1',88,'good','2026-10-31T03:00:00Z','2029-10-31T03:00:00Z')`,
		`INSERT INTO orders(order_no,student_id,course_id,course_name_snapshot,price_snapshot_cents,amount_cents,status,expired_at,paid_at,created_at) VALUES('O1',` + strconv.FormatInt(student.ID, 10) + `,1,'样例课程',1000,1000,'paid','2026-10-31T03:00:00Z','2026-10-31T02:00:00Z','2026-10-31T01:00:00Z')`,
		`INSERT INTO ai_daily_quota(student_id,course_id,stat_date,used) VALUES(` + strconv.FormatInt(student.ID, 10) + `,1,'2026-10-31',3)`,
		`INSERT INTO note(student_id,course_id,title,content_md) VALUES(` + strconv.FormatInt(student.ID, 10) + `,1,'私密笔记','管理员不可读的笔记正文')`,
	}
	for _, stmt := range stmts {
		if _, err := database.Exec(stmt); err != nil {
			database.Close()
			t.Fatal(err)
		}
	}
	server := NewWithIdentityAndDatabase(testConfig(t, true), service, database)
	r := server.Router()
	login := performJSON(t, r, http.MethodPost, "/api/v1/admin/login", `{"account":"boss","password":"Temp@123"}`)
	return server, database, extractJSONField(t, login.Body.String(), "token"), bossID
}

func TestAdminUserDetailStudentProgressAndMasking(t *testing.T) {
	server, database, token, _ := adminDetailServer(t)
	defer database.Close()
	r := server.Router()
	got := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/users/1?type=student", "", token)
	if got.Code != http.StatusOK {
		t.Fatalf("用户详情应可读: %d %s", got.Code, got.Body.String())
	}
	body := got.Body.String()
	for _, forbidden := range []string{"13812345678", "110101199001011237", "管理员不可读的笔记正文"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("详情不得返回敏感/笔记内容 %q: %s", forbidden, body)
		}
	}
	if !containsAll(body, `"phone_masked":"138****5678"`, `"id_card_masked":"110101********1237"`) {
		t.Fatalf("默认应脱敏: %s", body)
	}

	var resp struct {
		Data struct {
			Learning []struct {
				CourseCode      string `json:"course_code"`
				LectureCount    int    `json:"lecture_count"`
				LearnedLectures int    `json:"learned_lectures"`
				ProgressPercent int    `json:"progress_percent"`
				LastLectureNo   int    `json:"last_lecture_no"`
			} `json:"learning"`
			LearningSummary struct {
				Courses int `json:"courses"`
			} `json:"learning_summary"`
			Exams        []map[string]any `json:"exams"`
			Certificates []map[string]any `json:"certificates"`
			Orders       []map[string]any `json:"orders"`
			AIUsage      struct {
				UsedTotal int `json:"used_total"`
			} `json:"ai_usage"`
		} `json:"data"`
	}
	if err := json.Unmarshal(got.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	learning := resp.Data.Learning
	if len(learning) != 1 || learning[0].CourseCode != "C1" || learning[0].LectureCount != 2 ||
		learning[0].LearnedLectures != 1 || learning[0].ProgressPercent != 50 || learning[0].LastLectureNo != 2 {
		t.Fatalf("学习进度口径异常: %+v", learning)
	}
	if resp.Data.LearningSummary.Courses != 1 || len(resp.Data.Exams) != 1 ||
		len(resp.Data.Certificates) != 1 || len(resp.Data.Orders) != 1 || resp.Data.AIUsage.UsedTotal != 3 {
		t.Fatalf("详情聚合数据异常: %+v", resp.Data)
	}
}

func TestAdminUserDetailStaffAndErrors(t *testing.T) {
	server, database, token, bossID := adminDetailServer(t)
	defer database.Close()
	r := server.Router()
	if got := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/users/999?type=student", "", token); got.Code != http.StatusNotFound {
		t.Fatalf("不存在用户应 404，实际 %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/users/1?type=bad", "", token); got.Code != http.StatusBadRequest {
		t.Fatalf("非法类型应 400，实际 %d", got.Code)
	}
	staffDetail := performJSONWithBearer(t, r, http.MethodGet,
		"/api/v1/admin/users/"+strconv.FormatInt(bossID, 10)+"?type=admin_super", "", token)
	if staffDetail.Code != http.StatusOK || !containsAll(staffDetail.Body.String(), `"account":"boss"`, `"user_type":"admin_super"`) {
		t.Fatalf("内部人员详情异常: %d %s", staffDetail.Code, staffDetail.Body.String())
	}
	// 角色不匹配时也不能跨类型取到别人。
	teacherAsSuper := performJSONWithBearer(t, r, http.MethodGet,
		"/api/v1/admin/users/"+strconv.FormatInt(bossID, 10)+"?type=teacher", "", token)
	if teacherAsSuper.Code != http.StatusNotFound {
		t.Fatalf("类型不匹配应 404，实际 %d", teacherAsSuper.Code)
	}
}

func TestAdminUserListSearchAndFilters(t *testing.T) {
	server, database, token, _ := adminDetailServer(t)
	defer database.Close()
	r := server.Router()
	byPhone := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/users?type=student&q=1381", "", token)
	if byPhone.Code != http.StatusOK || !containsAll(byPhone.Body.String(), `"total":1`) {
		t.Fatalf("手机号前缀搜索失败: %d %s", byPhone.Code, byPhone.Body.String())
	}
	byName := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/users?q=样例", "", token)
	if byName.Code != http.StatusOK || !containsAll(byName.Body.String(), `"name":"样例学员"`) {
		t.Fatalf("姓名搜索失败: %d %s", byName.Code, byName.Body.String())
	}
	// 身份证号不能作为搜索入口（传了也必须被忽略）。
	byIDCard := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/users?q=110101199001011237", "", token)
	if byIDCard.Code != http.StatusOK || strings.Contains(byIDCard.Body.String(), `"total":1`) {
		t.Fatalf("身份证号不得用于搜索: %d %s", byIDCard.Code, byIDCard.Body.String())
	}
	suspended := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/users?status=suspended", "", token)
	if suspended.Code != http.StatusOK || !containsAll(suspended.Body.String(), `"total":0`) {
		t.Fatalf("状态筛选失败: %d %s", suspended.Code, suspended.Body.String())
	}
	if got := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/users?status=bad", "", token); got.Code != http.StatusBadRequest {
		t.Fatalf("非法状态应 400，实际 %d", got.Code)
	}
}
