package api

import (
	"net/http"
	"testing"
)

func TestAdminHandleCommentReportAndRestore(t *testing.T) {
	server, database, token, _ := adminDetailServer(t)
	defer database.Close()
	r := server.Router()
	if _, err := database.Exec(`INSERT INTO course_comment(course_code,student_id,rating,body,nickname_shown,status)
		VALUES('C1',1,1,'这是包含违规内容的评价正文，需要管理员复核处理。','学员***','visible')`); err != nil {
		t.Fatal(err)
	}
	var commentID int64
	if err := database.QueryRow(`SELECT id FROM course_comment LIMIT 1`).Scan(&commentID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO comment_report(comment_id,reporter_id,reason_kind) VALUES($1,1,'abuse')`, commentID); err != nil {
		t.Fatal(err)
	}
	var reportID int64
	if err := database.QueryRow(`SELECT id FROM comment_report WHERE comment_id=$1`, commentID).Scan(&reportID); err != nil {
		t.Fatal(err)
	}

	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/comment-reports/"+itoaTest(reportID)+"/handle",
		`{"result_note":"删除","remove_comment":true,"reason_kind":"差评","confirm":true}`, token); got.Code != http.StatusBadRequest {
		t.Fatalf("差评原因必须被拒: %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/comment-reports/"+itoaTest(reportID)+"/handle",
		`{"result_note":"","remove_comment":false,"confirm":true}`, token); got.Code != http.StatusBadRequest {
		t.Fatalf("缺少结论应 400，实际 %d", got.Code)
	}
	ok := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/comment-reports/"+itoaTest(reportID)+"/handle",
		`{"result_note":"已核实为辱骂，评论下架","remove_comment":true,"reason_kind":"abuse","confirm":true}`, token)
	if ok.Code != http.StatusOK {
		t.Fatalf("处理举报失败: %d %s", ok.Code, ok.Body.String())
	}
	var commentStatus, reportStatus string
	if err := database.QueryRow(`SELECT status FROM course_comment WHERE id=$1`, commentID).Scan(&commentStatus); err != nil || commentStatus != "removed" {
		t.Fatalf("评论应下架: %s %v", commentStatus, err)
	}
	if err := database.QueryRow(`SELECT status FROM comment_report WHERE id=$1`, reportID).Scan(&reportStatus); err != nil || reportStatus != "handled" {
		t.Fatalf("举报应已处理: %s %v", reportStatus, err)
	}
	var notices, audits int
	if err := database.QueryRow(`SELECT COUNT(*) FROM notification WHERE student_id=1 AND title='举报已处理'`).Scan(&notices); err != nil || notices != 1 {
		t.Fatalf("处理结果应通知举报人: %d %v", notices, err)
	}
	if err := database.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action IN ('comment_report.handle','comment.remove')`).Scan(&audits); err != nil || audits < 2 {
		t.Fatalf("举报处理与下架必须写审计: %d %v", audits, err)
	}
	if again := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/comment-reports/"+itoaTest(reportID)+"/handle",
		`{"result_note":"重复","remove_comment":false,"confirm":true}`, token); again.Code != http.StatusConflict {
		t.Fatalf("重复处理应 409，实际 %d", again.Code)
	}

	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/comments/"+itoaTest(commentID)+"/restore",
		`{"reason":"误判，恢复","confirm":false}`, token); got.Code != http.StatusBadRequest {
		t.Fatalf("缺少确认应 400，实际 %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/comments/"+itoaTest(commentID)+"/restore",
		`{"reason":"误判，恢复","confirm":true}`, token); got.Code != http.StatusOK {
		t.Fatalf("恢复评论失败: %d %s", got.Code, got.Body.String())
	}
	if err := database.QueryRow(`SELECT status FROM course_comment WHERE id=$1`, commentID).Scan(&commentStatus); err != nil || commentStatus != "visible" {
		t.Fatalf("恢复后评论应可见: %s %v", commentStatus, err)
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/comments/"+itoaTest(commentID)+"/restore",
		`{"reason":"重复","confirm":true}`, token); got.Code != http.StatusConflict {
		t.Fatalf("重复恢复应 409，实际 %d", got.Code)
	}
}
