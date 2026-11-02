package api

import (
	"net/http"
	"strings"
	"testing"
)

func TestAdminOrderLedgerAndExport(t *testing.T) {
	server, database, token, _ := adminDetailServer(t)
	defer database.Close()
	r := server.Router()
	list := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/orders?status=paid&course_code=C1&q=O1", "", token)
	if list.Code != http.StatusOK || !containsAll(list.Body.String(), `"total":1`, `"order_no":"O1"`, `"phone":"138****5678"`) {
		t.Fatalf("订单筛选异常: %d %s", list.Code, list.Body.String())
	}
	if strings.Contains(list.Body.String(), "13812345678") {
		t.Fatalf("订单台账不得返回明文手机号: %s", list.Body.String())
	}
	byDate := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/orders?from=2026-10-31&to=2026-10-31", "", token)
	if byDate.Code != http.StatusOK || !containsAll(byDate.Body.String(), `"total":1`) {
		t.Fatalf("日期筛选异常: %d %s", byDate.Code, byDate.Body.String())
	}
	for _, query := range []string{"?status=bad", "?from=2026/10/31", "?to=bad", "?limit=0", "?limit=101", "?offset=-1"} {
		if got := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/orders"+query, "", token); got.Code != http.StatusBadRequest {
			t.Fatalf("非法筛选 %s 应 400，实际 %d", query, got.Code)
		}
	}
	csvResp := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/orders/export?status=paid", "", token)
	if csvResp.Code != http.StatusOK || !strings.Contains(csvResp.Body.String(), "138****5678") || strings.Contains(csvResp.Body.String(), "13812345678") {
		t.Fatalf("订单 CSV 异常: %d %s", csvResp.Code, csvResp.Body.String())
	}
}

func TestAdminPartialRefundKeepsEntitlementAndCert(t *testing.T) {
	server, database, token, _ := adminDetailServer(t)
	defer database.Close()
	r := server.Router()
	for _, body := range []string{
		`{"amount_cents":0,"basis":"campaign","basis_ref":"X","reason":"补偿","confirm":true}`,
		`{"amount_cents":400,"basis":"","basis_ref":"X","reason":"补偿","confirm":true}`,
		`{"amount_cents":400,"basis":"campaign","basis_ref":"X","reason":"","confirm":true}`,
		`{"amount_cents":400,"basis":"campaign","basis_ref":"X","reason":"补偿","confirm":false}`,
	} {
		if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/orders/O1/partial-refund", body, token); got.Code != http.StatusBadRequest {
			t.Fatalf("非法部分退款应 400: %s -> %d", body, got.Code)
		}
	}
	ok := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/orders/O1/partial-refund",
		`{"amount_cents":400,"basis":"campaign","basis_ref":"活动-折扣","reason":"活动差价返还","confirm":true}`, token)
	if ok.Code != http.StatusOK || !containsAll(ok.Body.String(), `"kind":"reward"`, `"amount_cents":400`) {
		t.Fatalf("部分退款失败: %d %s", ok.Code, ok.Body.String())
	}
	var kind string
	var amount int64
	if err := database.QueryRow(`SELECT kind, amount_cents FROM refund WHERE order_id=(SELECT id FROM orders WHERE order_no='O1')`).Scan(&kind, &amount); err != nil || kind != "reward" || amount != 400 {
		t.Fatalf("退款记录异常: %s %d %v", kind, amount, err)
	}
	var entitlements, certs int
	if err := database.QueryRow(`SELECT COUNT(*) FROM entitlement WHERE student_id=1 AND course_id=1 AND revoked_at IS NULL`).Scan(&entitlements); err != nil || entitlements != 1 {
		t.Fatalf("部分退款应保留权益: %d %v", entitlements, err)
	}
	if err := database.QueryRow(`SELECT COUNT(*) FROM certificate WHERE student_id=1 AND course_id=1 AND status='valid'`).Scan(&certs); err != nil || certs != 1 {
		t.Fatalf("部分退款应保留证书: %d %v", certs, err)
	}
	var audits int
	if err := database.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action='order.partial_refund'`).Scan(&audits); err != nil || audits != 1 {
		t.Fatalf("部分退款必须写审计: %d %v", audits, err)
	}
	if again := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/orders/O1/partial-refund",
		`{"amount_cents":100,"basis":"campaign","basis_ref":"X","reason":"重复","confirm":true}`, token); again.Code != http.StatusConflict {
		t.Fatalf("重复部分退款应 409，实际 %d", again.Code)
	}
}
