package api

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/cw-tc/server/internal/identity"
	"github.com/cw-tc/server/internal/platform/db"
	"github.com/cw-tc/server/migrations"
)

func orderLifecycleServer(t *testing.T) (*Server, string, func()) {
	t.Helper()
	database, err := db.Open(context.Background(), db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.ApplyMigrations(context.Background(), database, migrations.FS, db.DialectSQLite); err != nil {
		t.Fatal(err)
	}
	service := identity.NewService(database, []byte("01234567890123456789012345678901"), []byte("hmac-key-for-cwTC-tests-012345678901234"))
	student, err := service.Register(context.Background(), identity.RegisterInput{Name: "样例学员", Phone: "13812345678", IDCard: "110101199001011237", Password: "password-1"})
	if err != nil {
		t.Fatal(err)
	}
	session, err := service.CreateSession(context.Background(), student.ID, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(`INSERT INTO staff(account,name,role,password_hash,created_source) VALUES('sample','样例教师','teacher','x','seed')`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(`INSERT INTO major(name) VALUES('样例专业')`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(`INSERT INTO course(course_code,source,name,line,major_id,status,teacher_id,price_cents) VALUES('C1','platform','样例课程','main',1,'published',1,12900)`); err != nil {
		t.Fatal(err)
	}
	return NewWithIdentityAndDatabase(testConfig(t, true), service, database), session.Token, func() { database.Close() }
}

func TestOrderLifecycleRoutesQueryPayAndCancel(t *testing.T) {
	s, token, cleanup := orderLifecycleServer(t)
	defer cleanup()
	r := s.Router()
	created := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/orders", `{"course_code":"C1"}`, token)
	orderNo := extractJSONField(t, created.Body.String(), "order_no")
	query := performJSONWithCookie(t, r, http.MethodGet, "/api/v1/orders/"+orderNo, ``, token)
	if query.Code != http.StatusOK || !containsAll(query.Body.String(), `"status":"pending"`) {
		t.Fatalf("订单查询失败: %d %s", query.Code, query.Body.String())
	}
	pay := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/orders/"+orderNo+"/pay", `{"channel":"mock","amount_cents":12900}`, token)
	if pay.Code != http.StatusOK || !containsAll(pay.Body.String(), `"provider":"mock-not-configured"`) {
		t.Fatalf("预下单失败: %d %s", pay.Code, pay.Body.String())
	}
	cancel := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/orders/"+orderNo+"/cancel", `{}`, token)
	if cancel.Code != http.StatusOK {
		t.Fatalf("取消订单失败: %d %s", cancel.Code, cancel.Body.String())
	}
}

func TestMockPayGrantsEntitlement(t *testing.T) {
	s, token, cleanup := orderLifecycleServer(t)
	defer cleanup()
	r := s.Router()
	// 不存在的课程码 → 404
	if bad := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/orders", `{"course_code":"NOPE"}`, token); bad.Code != http.StatusNotFound {
		t.Fatalf("不存在课程下单应 404，实际 %d", bad.Code)
	}
	created := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/orders", `{"course_code":"C1"}`, token)
	orderNo := extractJSONField(t, created.Body.String(), "order_no")
	// 模拟支付成功
	pay := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/orders/"+orderNo+"/mock-pay", `{}`, token)
	if pay.Code != http.StatusOK || !containsAll(pay.Body.String(), `"status":"paid"`) {
		t.Fatalf("模拟支付失败: %d %s", pay.Code, pay.Body.String())
	}
	// 权益已开通
	ents := performJSONWithCookie(t, r, http.MethodGet, "/api/v1/me/entitlements", ``, token)
	if ents.Code != http.StatusOK || !containsAll(ents.Body.String(), `"course_id"`) {
		t.Fatalf("权益未开通: %d %s", ents.Code, ents.Body.String())
	}
	// 重复 mock-pay → 订单已非 pending，应报错（幂等）
	pay2 := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/orders/"+orderNo+"/mock-pay", `{}`, token)
	if pay2.Code != http.StatusBadRequest {
		t.Fatalf("重复 mock-pay 应 400，实际 %d", pay2.Code)
	}
}
