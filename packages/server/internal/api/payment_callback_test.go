package api

import (
	"context"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/cw-tc/server/internal/commerce"
	"github.com/cw-tc/server/internal/identity"
	"github.com/cw-tc/server/internal/platform/db"
	"github.com/cw-tc/server/migrations"
)

func TestPaymentCallbackRouteCompletesFirstOrder(t *testing.T) {
	database, err := db.Open(context.Background(), db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
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
	if _, err = database.Exec(`INSERT INTO course(course_code,source,name,line,major_id,status,teacher_id,price_cents) VALUES('C1','platform','样例课程','main',1,'published',1,1000)`); err != nil {
		t.Fatal(err)
	}
	cfg := testConfig(t, true)
	cfg.WXPayAPIv3Key = "callback-test-secret"
	r := NewWithIdentityAndDatabase(cfg, service, database).Router()
	created := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/orders", `{"course_code":"C1"}`, session.Token)
	orderNo := extractJSONField(t, created.Body.String(), "order_no")
	signature := commerce.SignCallback([]byte("callback-test-secret"), orderNo, "txn-http-1", "1000")
	callback := performJSON(t, r, http.MethodPost, "/api/v1/payment/callback", `{"order_no":"`+orderNo+`","channel":"mock","channel_txn_id":"txn-http-1","idempotency_key":"idem-http-1","amount_cents":1000,"signature":"`+signature+`"}`)
	if callback.Code != http.StatusOK {
		t.Fatalf("支付回调失败: %d %s", callback.Code, callback.Body.String())
	}
	entitlements := performJSONWithCookie(t, r, http.MethodGet, "/api/v1/me/entitlements", ``, session.Token)
	if entitlements.Code != http.StatusOK || !containsAll(entitlements.Body.String(), `"course_id":1`, `"status":"active"`) {
		t.Fatalf("首单权益未开通: %d %s", entitlements.Code, entitlements.Body.String())
	}
	_ = strconv.IntSize
}
