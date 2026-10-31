package api

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/cw-tc/server/internal/identity"
)

func TestAdminAppealListAndHandleUnbans(t *testing.T) {
	server, database, token, _ := adminDetailServer(t)
	defer database.Close()
	r := server.Router()
	if _, err := database.Exec(`UPDATE student SET status='suspended', status_reason_kind='serious_violation', status_note='倒卖课程' WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO account_appeal(student_id, phone, kind, content_text, status, created_at) VALUES(1,'13812345678','ban','账号被盗用，申请复核解封。','pending','2026-10-31T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	list := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/appeals?status=pending", "", token)
	if list.Code != http.StatusOK || !containsAll(list.Body.String(), `"phone_masked":"138****5678"`, `"kind":"ban"`, `"name":"样例学员"`) {
		t.Fatalf("申诉待办异常: %d %s", list.Code, list.Body.String())
	}
	if strings.Contains(list.Body.String(), "13812345678") {
		t.Fatalf("申诉列表不得返回明文手机号: %s", list.Body.String())
	}
	for _, body := range []string{
		`{"status":"accepted","result_note":"","confirm":true}`,
		`{"status":"accepted","result_note":"通过","confirm":false}`,
		`{"status":"bad","result_note":"x","confirm":true}`,
	} {
		if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/appeals/1/handle", body, token); got.Code != http.StatusBadRequest {
			t.Fatalf("非法处理应 400: %s -> %d", body, got.Code)
		}
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/appeals/999/handle",
		`{"status":"rejected","result_note":"不存在","confirm":true}`, token); got.Code != http.StatusNotFound {
		t.Fatalf("不存在申诉应 404，实际 %d", got.Code)
	}
	ok := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/appeals/1/handle",
		`{"status":"accepted","result_note":"核实为异地登录，已解封","confirm":true}`, token)
	if ok.Code != http.StatusOK || !containsAll(ok.Body.String(), `"status":"accepted"`) {
		t.Fatalf("处理申诉失败: %d %s", ok.Code, ok.Body.String())
	}
	var status string
	if err := database.QueryRow(`SELECT status FROM student WHERE id=1`).Scan(&status); err != nil || status != "active" {
		t.Fatalf("封禁申诉通过应自动解封: %s %v", status, err)
	}
	var notices, audits int
	if err := database.QueryRow(`SELECT COUNT(*) FROM notification WHERE student_id=1 AND title='申诉已受理'`).Scan(&notices); err != nil || notices != 1 {
		t.Fatalf("处理结果应通知学员: %d %v", notices, err)
	}
	if err := database.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action IN ('account_appeal.handle','user.status_change')`).Scan(&audits); err != nil || audits < 2 {
		t.Fatalf("申诉处理与解封必须写审计: %d %v", audits, err)
	}
	if again := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/appeals/1/handle",
		`{"status":"rejected","result_note":"重复","confirm":true}`, token); again.Code != http.StatusConflict {
		t.Fatalf("重复处理应 409，实际 %d", again.Code)
	}
}

func TestStudentAndAnonymousAppeals(t *testing.T) {
	server, database, _, _ := adminDetailServer(t)
	defer database.Close()
	r := server.Router()
	session, err := server.identity.CreateSession(context.Background(), 1, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if got := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/me/appeals", `{"kind":"score","content_text":"太短"}`, session.Token); got.Code != http.StatusBadRequest {
		t.Fatalf("过短申诉应 400，实际 %d", got.Code)
	}
	created := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/me/appeals",
		`{"kind":"score","content_text":"第三题判分与参考答案不一致，申请复核。"}`, session.Token)
	if created.Code != http.StatusOK {
		t.Fatalf("学员提交申诉失败: %d %s", created.Code, created.Body.String())
	}
	mine := performJSONWithCookie(t, r, http.MethodGet, "/api/v1/me/appeals", ``, session.Token)
	if mine.Code != http.StatusOK || !containsAll(mine.Body.String(), `"kind":"score"`, `"status":"pending"`) {
		t.Fatalf("我的申诉列表异常: %d %s", mine.Code, mine.Body.String())
	}

	// 未配置短信服务时，登录页申诉必须 fail-closed。
	if got := performJSON(t, r, http.MethodPost, "/api/v1/auth/appeal",
		`{"phone":"13812345678","code":"123456","kind":"ban","content_text":"账号被误封，申请复核。"}`); got.Code != http.StatusServiceUnavailable {
		t.Fatalf("短信未配置时申诉应 503，实际 %d", got.Code)
	}
	sms := identity.NewSMSService()
	server.WithSMS(sms)
	if _, err := sms.Issue(context.Background(), "13812345678", "appeal", "654321", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if got := performJSON(t, r, http.MethodPost, "/api/v1/auth/appeal",
		`{"phone":"13812345678","code":"000000","kind":"ban","content_text":"账号被误封，申请复核。"}`); got.Code != http.StatusUnauthorized {
		t.Fatalf("错误验证码应 401，实际 %d", got.Code)
	}
	if _, err := sms.Issue(context.Background(), "13912345678", "appeal", "654321", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	anon := performJSON(t, r, http.MethodPost, "/api/v1/auth/appeal",
		`{"phone":"13912345678","code":"654321","kind":"ban","content_text":"账号被误封，申请复核。"}`)
	if anon.Code != http.StatusOK || !containsAll(anon.Body.String(), `"status":"pending"`) {
		t.Fatalf("匿名申诉失败: %d %s", anon.Code, anon.Body.String())
	}
}
