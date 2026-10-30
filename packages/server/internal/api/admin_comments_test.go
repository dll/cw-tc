package api

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/cw-tc/server/internal/staff"
)

func TestAdminCommentModerationAndReport(t *testing.T) {
	s, token, cleanup := orderLifecycleServer(t)
	defer cleanup()
	r := s.Router()
	if _, err := staff.Create(context.Background(), s.dbConn, "admin2", "管理员", staff.RoleAdminOps, "Temp@123", 0); err != nil {
		t.Fatal(err)
	}
	login := performJSON(t, r, http.MethodPost, "/api/v1/admin/login", `{"account":"admin2","password":"Temp@123"}`)
	adminToken := extractJSONField(t, login.Body.String(), "token")
	var studentID, courseID int64
	_ = s.dbConn.QueryRow(`SELECT id FROM student LIMIT 1`).Scan(&studentID)
	_ = s.dbConn.QueryRow(`SELECT id FROM course WHERE course_code='C1'`).Scan(&courseID)
	res, err := s.dbConn.Exec(`INSERT INTO course_comment(course_code,student_id,rating,body,completed_lectures,total_lectures,nickname_shown,status) VALUES('C1',?,5,'这是一条用于审核的课程评价内容，长度必须超过二十个字符才能通过数据库约束。',1,2,'学员***','visible')`, studentID)
	if err != nil {
		t.Fatal(err)
	}
	commentID, _ := res.LastInsertId()
	listed := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/comments?status=visible", "", adminToken)
	if listed.Code != http.StatusOK || !strings.Contains(listed.Body.String(), `"course_code":"C1"`) {
		t.Fatalf("评价审核列表异常: %d %s", listed.Code, listed.Body.String())
	}
	report := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/courses/C1/comments/"+strconv.FormatInt(commentID, 10)+"/report", `{"reason_kind":"广告"}`, token)
	if report.Code != http.StatusOK {
		t.Fatalf("举报失败: %d %s", report.Code, report.Body.String())
	}
	removed := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/comments/"+strconv.FormatInt(commentID, 10)+"/remove", `{"reason_kind":"广告","confirm":true}`, adminToken)
	if removed.Code != http.StatusOK {
		t.Fatalf("下架评价失败: %d %s", removed.Code, removed.Body.String())
	}
	public := performJSON(t, r, http.MethodGet, "/api/v1/courses/C1/comments", "")
	if public.Code != http.StatusOK || strings.Contains(public.Body.String(), `"count":1`) {
		t.Fatalf("下架后不应可见: %d %s", public.Code, public.Body.String())
	}
}
