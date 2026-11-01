package api

import (
	"context"
	"net/http"
	"testing"
)

func TestDeletionRequestReviewFlow(t *testing.T) {
	server, database, adminToken, _ := adminDetailServer(t)
	defer database.Close()
	r := server.Router()
	session, err := server.identity.CreateSession(context.Background(), 1, timeNowUTC())
	if err != nil {
		t.Fatal(err)
	}
	request := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/me/delete", `{"reason":"不再使用平台"}`, session.Token)
	if request.Code != http.StatusOK || !containsAll(request.Body.String(), `"status":"deleting"`) {
		t.Fatalf("注销申请提交失败: %d %s", request.Code, request.Body.String())
	}
	var status string
	if err := database.QueryRow(`SELECT status FROM student WHERE id=1`).Scan(&status); err != nil || status != "deleting" {
		t.Fatalf("申请后账号应进入 deleting: %s %v", status, err)
	}
	list := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/deletions", "", adminToken)
	if list.Code != http.StatusOK || !containsAll(list.Body.String(), `"status":"pending"`, `"reason":"不再使用平台"`) {
		t.Fatalf("注销待办异常: %d %s", list.Code, list.Body.String())
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/deletions/1/approve",
		`{"approve":false,"note":"","confirm":true}`, adminToken); got.Code != http.StatusBadRequest {
		t.Fatalf("缺少结论应 400，实际 %d", got.Code)
	}
	reject := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/deletions/1/approve",
		`{"approve":false,"note":"资料尚在核对，暂不注销","confirm":true}`, adminToken)
	if reject.Code != http.StatusOK {
		t.Fatalf("驳回注销失败: %d %s", reject.Code, reject.Body.String())
	}
	if err := database.QueryRow(`SELECT status FROM student WHERE id=1`).Scan(&status); err != nil || status != "active" {
		t.Fatalf("驳回后账号应恢复 active: %s %v", status, err)
	}
	var notices int
	if err := database.QueryRow(`SELECT COUNT(*) FROM notification WHERE student_id=1 AND title='注销申请未通过'`).Scan(&notices); err != nil || notices != 1 {
		t.Fatalf("驳回应通知学员: %d %v", notices, err)
	}

	// 再次申请并批准 → 匿名化；证书台账保留。
	second, err := server.identity.CreateSession(context.Background(), 1, timeNowUTC())
	if err != nil {
		t.Fatal(err)
	}
	if got := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/me/delete", `{"reason":"确认注销"}`, second.Token); got.Code != http.StatusOK {
		t.Fatalf("再次申请失败: %d %s", got.Code, got.Body.String())
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/deletions/2/approve",
		`{"approve":true,"note":"已核对无进行中订单与退款","confirm":true}`, adminToken); got.Code != http.StatusOK {
		t.Fatalf("批准注销失败: %d %s", got.Code, got.Body.String())
	}
	var name, studentStatus string
	var cipher []byte
	if err := database.QueryRow(`SELECT name, status, id_card_cipher FROM student WHERE id=1`).Scan(&name, &studentStatus, &cipher); err != nil {
		t.Fatal(err)
	}
	if name != "已注销" || studentStatus != "anonymized" || len(cipher) != 0 {
		t.Fatalf("匿名化结果异常: %s %s %d", name, studentStatus, len(cipher))
	}
	var certs int
	if err := database.QueryRow(`SELECT COUNT(*) FROM certificate WHERE student_id=1`).Scan(&certs); err != nil || certs != 1 {
		t.Fatalf("注销后证书台账应保留: %d %v", certs, err)
	}
	var audits int
	if err := database.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action IN ('account.deletion_request','deletion.reject','deletion.approve')`).Scan(&audits); err != nil || audits != 4 {
		t.Fatalf("注销流程审计异常: %d %v", audits, err)
	}
}

func TestDeletionRequestBlockedByPendingOrder(t *testing.T) {
	server, database, _, _ := adminDetailServer(t)
	defer database.Close()
	if _, err := database.Exec(`INSERT INTO orders(order_no,student_id,course_id,course_name_snapshot,price_snapshot_cents,amount_cents,status,expired_at,created_at)
		VALUES('O-PENDING',1,1,'样例课程',1000,1000,'pending','2099-01-01T00:00:00Z','2026-10-31T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	session, err := server.identity.CreateSession(context.Background(), 1, timeNowUTC())
	if err != nil {
		t.Fatal(err)
	}
	if got := performJSONWithCookie(t, server.Router(), http.MethodPost, "/api/v1/me/delete", `{}`, session.Token); got.Code != http.StatusConflict {
		t.Fatalf("有待支付订单应拒绝注销，实际 %d %s", got.Code, got.Body.String())
	}
	var requests int
	if err := database.QueryRow(`SELECT COUNT(*) FROM deletion_request`).Scan(&requests); err != nil || requests != 0 {
		t.Fatalf("被拒绝时不应写注销申请: %d %v", requests, err)
	}
}
