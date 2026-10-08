package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/cw-tc/server/internal/platform/db"
	"github.com/cw-tc/server/internal/staff"
	"github.com/cw-tc/server/migrations"
)

func TestTeacherGradeRoutes(t *testing.T) {
	database, err := db.Open(context.Background(), db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := db.ApplyMigrations(context.Background(), database, migrations.FS, db.DialectSQLite); err != nil {
		t.Fatal(err)
	}
	teacherID, err := staff.Create(context.Background(), database, "t", "教师", staff.RoleTeacher, "Temp@123", 0)
	if err != nil {
		t.Fatal(err)
	}
	otherID, _ := staff.Create(context.Background(), database, "t2", "乙教师", staff.RoleTeacher, "Temp@123", 0)
	_ = otherID
	if _, err := database.Exec(`INSERT INTO major(id, name) VALUES(1,'样例专业')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO course(id, course_code, source, name, line, major_id, status, teacher_id, cert_pass_lines) VALUES(1,'C1','platform','样例课程','main',1,'published',?,'{"L1":3.0}')`, teacherID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO student(id, student_no, name, phone, id_card_hash, id_card_cipher, agreement_versions) VALUES(1,'S00000001','样例学员','13812345678',X'01',X'02','[]')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO question(id, course_id, qno, level, type, gradability, stem, answer, score) VALUES(1,1,'L1-02-01','L1','short','manual','简述指针作用','间接访问内存',1.5)`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO exam_session(id, student_id, course_id, level, paper, answers, status, started_at, server_deadline_at) VALUES(1,1,1,'L1','[{"qno":"L1-02-01","score":1.5}]','{"L1-02-01":"间接访问"}','graded','2026-01-01T00:00:00Z','2026-01-01T01:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO exam_result(id, exam_session_id, student_id, course_id, level, auto_score, manual_score, total_score, band, is_pass, detail) VALUES(1,1,1,1,'L1',0,0,0,'fail',false,'{"L1-02-01":{"manual":true,"score":0}}')`); err != nil {
		t.Fatal(err)
	}
	r := NewWithDatabase(testConfig(t, true), database).Router()

	login := performJSON(t, r, http.MethodPost, "/api/v1/admin/login", `{"account":"t","password":"Temp@123"}`)
	token := extractJSONField(t, login.Body.String(), "token")

	// 待阅卷清单
	list := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/teacher/grade", "", token)
	if list.Code != http.StatusOK || !containsAll(list.Body.String(), `"ResultID":1`) {
		t.Fatalf("待阅卷清单 = %d %s", list.Code, list.Body.String())
	}
	// 逐题详情（含题干与作答）
	detail := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/teacher/grade/1", "", token)
	if detail.Code != http.StatusOK || !containsAll(detail.Body.String(), `"qno":"L1-02-01"`, `"question_id":1`, `"stem":"简述指针作用"`) {
		t.Fatalf("逐题详情 = %d %s", detail.Code, detail.Body.String())
	}
	// 评阅主观题
	grade := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/teacher/grade/1/questions/1", `{"score":1.5,"comment":"要点齐全"}`, token)
	if grade.Code != http.StatusOK {
		t.Fatalf("评阅 = %d %s", grade.Code, grade.Body.String())
	}
	// 错误路径：负分 → 400；不存在结果 → 404；坏 id → 400
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/teacher/grade/1/questions/1", `{"score":-1}`, token); got.Code != http.StatusBadRequest {
		t.Fatalf("负分应 400，实际 %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/teacher/grade/999", "", token); got.Code != http.StatusNotFound {
		t.Fatalf("不存在结果应 404，实际 %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/teacher/grade/abc", "", token); got.Code != http.StatusBadRequest {
		t.Fatalf("坏 id 应 400，实际 %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/teacher/grade/abc/questions/1", `{"score":1}`, token); got.Code != http.StatusBadRequest {
		t.Fatalf("坏 id 应 400，实际 %d", got.Code)
	}
	// 他人课程评阅 → 403（归属校验，铁律 8）
	login2 := performJSON(t, r, http.MethodPost, "/api/v1/admin/login", `{"account":"t2","password":"Temp@123"}`)
	token2 := extractJSONField(t, login2.Body.String(), "token")
	if got := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/teacher/grade/1", "", token2); got.Code != http.StatusForbidden {
		t.Fatalf("他人课程应 403，实际 %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/teacher/grade/1/questions/1", `{"score":1}`, token2); got.Code != http.StatusForbidden {
		t.Fatalf("他人课程评阅应 403，实际 %d", got.Code)
	}
}
