package api

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestAdminPromoAndStudentOrderFlow(t *testing.T) {
	server, database, adminToken, _ := adminDetailServer(t)
	defer database.Close()
	r := server.Router()
	from := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)
	to := time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339)
	create := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/promo",
		fmt.Sprintf(`{"code":"P300","kind":"amount_off","value":300,"total_limit":10,"per_student_limit":1,"valid_from":"%s","valid_to":"%s","confirm":true}`, from, to), adminToken)
	if create.Code != http.StatusOK {
		t.Fatalf("创建优惠码失败: %d %s", create.Code, create.Body.String())
	}
	if list := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/promo", "", adminToken); list.Code != http.StatusOK || !containsAll(list.Body.String(), `"code":"P300"`, `"used":0`) {
		t.Fatalf("优惠码列表异常: %d %s", list.Code, list.Body.String())
	}
	session, err := server.identity.CreateSession(context.Background(), 1, timeNowUTC())
	if err != nil {
		t.Fatal(err)
	}
	validate := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/promo/validate", `{"code":"P300","course_code":"C1"}`, session.Token)
	if validate.Code != http.StatusOK || !containsAll(validate.Body.String(), `"discount_cents":300`, `"amount_cents":700`) {
		t.Fatalf("优惠码校验异常: %d %s", validate.Code, validate.Body.String())
	}
	order := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/orders", `{"course_code":"C1","promo_code":"P300"}`, session.Token)
	if order.Code != http.StatusOK || !containsAll(order.Body.String(), `"amount_cents":700`, `"discount_cents":300`) {
		t.Fatalf("优惠码下单异常: %d %s", order.Code, order.Body.String())
	}
	if again := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/orders", `{"course_code":"C1","promo_code":"P300"}`, session.Token); again.Code != http.StatusBadRequest {
		t.Fatalf("待支付订单存在时应拒绝新优惠码订单: %d", again.Code)
	}
	list := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/promo", "", adminToken)
	if list.Code != http.StatusOK || !containsAll(list.Body.String(), `"used":1`, `"discounted_cents":300`) {
		t.Fatalf("优惠码使用统计异常: %d %s", list.Code, list.Body.String())
	}
	if got := performJSONWithBearer(t, r, http.MethodPatch, "/api/v1/admin/promo/P300", `{"status":"disabled","confirm":true}`, adminToken); got.Code != http.StatusOK {
		t.Fatalf("停用优惠码失败: %d", got.Code)
	}
	if got := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/promo/validate", `{"code":"P300","course_code":"C1"}`, session.Token); got.Code != http.StatusBadRequest {
		t.Fatalf("停用后校验应失败: %d", got.Code)
	}
	for _, body := range []string{
		`{"code":"","kind":"amount_off","value":1,"valid_from":"2026-01-01T00:00:00Z","valid_to":"2026-02-01T00:00:00Z","confirm":true}`,
		`{"code":"X","kind":"bad","value":1,"valid_from":"2026-01-01T00:00:00Z","valid_to":"2026-02-01T00:00:00Z","confirm":true}`,
		`{"code":"X","kind":"amount_off","value":1,"valid_from":"2026-02-01T00:00:00Z","valid_to":"2026-01-01T00:00:00Z","confirm":true}`,
	} {
		if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/promo", body, adminToken); got.Code != http.StatusBadRequest {
			t.Fatalf("非法优惠码应 400: %s -> %d", body, got.Code)
		}
	}
}
