package api

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/cw-tc/server/internal/identity"
)

func TestMyOrdersListContinuePayAndRefund(t *testing.T) {
	s, token, cleanup := orderLifecycleServer(t)
	defer cleanup()
	r := s.Router()

	created := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/orders", `{"course_code":"C1"}`, token)
	orderNo := extractJSONField(t, created.Body.String(), "order_no")

	listed := performJSONWithCookie(t, r, http.MethodGet, "/api/v1/me/orders", ``, token)
	if listed.Code != http.StatusOK || !containsAll(listed.Body.String(),
		`"order_no":"`+orderNo+`"`, `"course_code":"C1"`, `"status":"pending"`,
		`"can_continue_pay":true`, `"can_cancel":true`, `"can_refund":false`) {
		t.Fatalf("订单列表异常: %d %s", listed.Code, listed.Body.String())
	}
	detail := performJSONWithCookie(t, r, http.MethodGet, "/api/v1/me/orders/"+orderNo, ``, token)
	if detail.Code != http.StatusOK || !strings.Contains(detail.Body.String(), `"order_no":"`+orderNo+`"`) {
		t.Fatalf("订单详情异常: %d %s", detail.Code, detail.Body.String())
	}
	if got := performJSONWithCookie(t, r, http.MethodGet, "/api/v1/me/orders/"+orderNo+"/refund", ``, token); got.Code != http.StatusNotFound {
		t.Fatalf("退款前查询应 404: %d %s", got.Code, got.Body.String())
	}
	if got := performJSONWithCookie(t, r, http.MethodGet, "/api/v1/me/orders/NOPE", ``, token); got.Code != http.StatusNotFound {
		t.Fatalf("不存在订单应 404: %d", got.Code)
	}

	pay := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/me/orders/"+orderNo+"/pay", `{}`, token)
	if pay.Code != http.StatusOK || !containsAll(pay.Body.String(), `"order_no":"`+orderNo+`"`, `"channel":"mock"`) {
		t.Fatalf("继续支付应复用原订单: %d %s", pay.Code, pay.Body.String())
	}
	if got := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/orders/"+orderNo+"/mock-pay", `{}`, token); got.Code != http.StatusOK {
		t.Fatalf("模拟支付失败: %d %s", got.Code, got.Body.String())
	}

	paid := performJSONWithCookie(t, r, http.MethodGet, "/api/v1/me/orders", ``, token)
	if paid.Code != http.StatusOK || !containsAll(paid.Body.String(),
		`"status":"paid"`, `"can_continue_pay":false`, `"can_enter_course":true`,
		`"can_refund":true`, `"cert_issued":false`, `"refundable_until":`) {
		t.Fatalf("已支付订单可操作标记异常: %d %s", paid.Code, paid.Body.String())
	}

	refunded := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/me/orders/"+orderNo+"/refund", `{}`, token)
	if refunded.Code != http.StatusOK || !containsAll(refunded.Body.String(), `"refund_no":"`, `"status":"success"`) {
		t.Fatalf("自助退款失败: %d %s", refunded.Code, refunded.Body.String())
	}
	after := performJSONWithCookie(t, r, http.MethodGet, "/api/v1/me/orders", ``, token)
	if after.Code != http.StatusOK || !containsAll(after.Body.String(),
		`"status":"refunded"`, `"can_enter_course":false`, `"can_refund":false`,
		`"refund":{"refund_no":"`, `"status":"success"`) {
		t.Fatalf("退款后订单状态异常: %d %s", after.Code, after.Body.String())
	}
	refundStatus := performJSONWithCookie(t, r, http.MethodGet, "/api/v1/me/orders/"+orderNo+"/refund", ``, token)
	if refundStatus.Code != http.StatusOK || !strings.Contains(refundStatus.Body.String(), `"status":"success"`) {
		t.Fatalf("退款进度异常: %d %s", refundStatus.Code, refundStatus.Body.String())
	}
	ents := performJSONWithCookie(t, r, http.MethodGet, "/api/v1/me/entitlements", ``, token)
	if strings.Contains(ents.Body.String(), `"course_code":"C1"`) {
		t.Fatalf("退款后权益未撤销: %s", ents.Body.String())
	}
	if got := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/me/orders/"+orderNo+"/refund", `{}`, token); got.Code != http.StatusConflict {
		t.Fatalf("重复退款应 409: %d %s", got.Code, got.Body.String())
	}
}

func TestMyOrdersCancelAndOwnership(t *testing.T) {
	s, token, cleanup := orderLifecycleServer(t)
	defer cleanup()
	r := s.Router()
	created := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/orders", `{"course_code":"C1"}`, token)
	orderNo := extractJSONField(t, created.Body.String(), "order_no")

	otherService := identity.NewService(s.dbConn, []byte("01234567890123456789012345678901"), []byte("hmac-key-for-cwTC-tests-012345678901234"))
	other, err := otherService.Register(context.Background(), identity.RegisterInput{Name: "样例他人", Phone: "13812345679", IDCard: "110101199003076755", Password: "password-2"})
	if err != nil {
		t.Fatal(err)
	}
	otherSession, err := otherService.CreateSession(context.Background(), other.ID, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if got := performJSONWithCookie(t, r, http.MethodGet, "/api/v1/me/orders", ``, otherSession.Token); got.Code != http.StatusOK || strings.Contains(got.Body.String(), orderNo) {
		t.Fatalf("他人不应看到该订单: %d %s", got.Code, got.Body.String())
	}
	if got := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/me/orders/"+orderNo+"/refund", `{}`, otherSession.Token); got.Code != http.StatusNotFound {
		t.Fatalf("他人退款应 404: %d %s", got.Code, got.Body.String())
	}

	if got := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/me/orders/"+orderNo+"/cancel", `{}`, token); got.Code != http.StatusOK {
		t.Fatalf("取消订单失败: %d %s", got.Code, got.Body.String())
	}
	closed := performJSONWithCookie(t, r, http.MethodGet, "/api/v1/me/orders", ``, token)
	if closed.Code != http.StatusOK || !containsAll(closed.Body.String(),
		`"status":"closed"`, `"can_cancel":false`, `"can_continue_pay":false`, `"can_refund":false`) {
		t.Fatalf("已关闭订单状态异常: %d %s", closed.Code, closed.Body.String())
	}
	if got := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/orders/"+orderNo+"/cancel", `{}`, token); got.Code != http.StatusBadRequest {
		t.Fatalf("重复取消应 400: %d", got.Code)
	}
	if got := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/me/orders/"+orderNo+"/pay", `{}`, token); got.Code != http.StatusConflict {
		t.Fatalf("已关闭订单继续支付应 409: %d %s", got.Code, got.Body.String())
	}
}

func TestMyOrdersRefundRejectsIssuedCertificateAndExpiredWindow(t *testing.T) {
	s, token, cleanup := orderLifecycleServer(t)
	defer cleanup()
	r := s.Router()

	created := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/orders", `{"course_code":"C1"}`, token)
	orderNo := extractJSONField(t, created.Body.String(), "order_no")
	if got := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/orders/"+orderNo+"/mock-pay", `{}`, token); got.Code != http.StatusOK {
		t.Fatalf("模拟支付失败: %d %s", got.Code, got.Body.String())
	}
	var studentID, courseID int64
	if err := s.dbConn.QueryRow(`SELECT o.student_id,o.course_id FROM orders o WHERE o.order_no=$1`, orderNo).Scan(&studentID, &courseID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.dbConn.Exec(`INSERT INTO certificate(cert_no,student_id,course_id,level,score,band,issued_at,expires_at) VALUES('CERT-1',$1,$2,'L1',90,'A','2026-01-01T00:00:00Z','2029-01-01T00:00:00Z')`, studentID, courseID); err != nil {
		t.Fatal(err)
	}
	blocked := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/me/orders/"+orderNo+"/refund", `{}`, token)
	if blocked.Code != http.StatusConflict || !strings.Contains(blocked.Body.String(), `"code":10305`) {
		t.Fatalf("已发证退款应 10305: %d %s", blocked.Code, blocked.Body.String())
	}

	if _, err := s.dbConn.Exec(`INSERT INTO course(course_code,source,name,line,status,price_cents) VALUES('C2','upstream','超窗课程','main','published',1000)`); err != nil {
		t.Fatal(err)
	}
	second := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/orders", `{"course_code":"C2"}`, token)
	secondNo := extractJSONField(t, second.Body.String(), "order_no")
	if got := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/orders/"+secondNo+"/mock-pay", `{}`, token); got.Code != http.StatusOK {
		t.Fatalf("第二单模拟支付失败: %d %s", got.Code, got.Body.String())
	}
	oldPaid := time.Now().UTC().Add(-8 * 24 * time.Hour).Format(time.RFC3339)
	if _, err := s.dbConn.Exec(`UPDATE orders SET paid_at=$1 WHERE order_no=$2`, oldPaid, secondNo); err != nil {
		t.Fatal(err)
	}
	expired := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/me/orders/"+secondNo+"/refund", `{}`, token)
	if expired.Code != http.StatusConflict || !strings.Contains(expired.Body.String(), `"code":10305`) {
		t.Fatalf("超窗退款应 10305: %d %s", expired.Code, expired.Body.String())
	}
	listed := performJSONWithCookie(t, r, http.MethodGet, "/api/v1/me/orders", ``, token)
	if listed.Code != http.StatusOK || !containsAll(listed.Body.String(), `"can_refund":false`) || strings.Contains(listed.Body.String(), `"can_refund":true`) {
		t.Fatalf("超窗订单不应显示可退款: %d %s", listed.Code, listed.Body.String())
	}
}
