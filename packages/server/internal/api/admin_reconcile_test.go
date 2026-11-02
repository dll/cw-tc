package api

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/cw-tc/server/internal/commerce"
)

func TestAdminReconcileAndChannels(t *testing.T) {
	server, database, token, _ := adminDetailServer(t)
	defer database.Close()
	r := server.Router()
	ctx := context.Background()
	svc := commerce.NewService(database)
	if _, err := svc.Reconcile(ctx, "2026-10-31", "wechat", 1, 1000); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Reconcile(ctx, "2026-11-01", "alipay", 2, 5000); err != nil {
		t.Fatal(err)
	}

	list := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/reconcile", "", token)
	if list.Code != http.StatusOK || !containsAll(list.Body.String(), `"all_balanced":false`, `"channel":"wechat"`, `"channel":"alipay"`) {
		t.Fatalf("对账列表异常: %d %s", list.Code, list.Body.String())
	}
	day := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/reconcile/2026-10-31", "", token)
	if day.Code != http.StatusOK || !containsAll(day.Body.String(), `"all_balanced":true`, `"diff_amount_cents":0`) {
		t.Fatalf("按日对账异常: %d %s", day.Code, day.Body.String())
	}
	csvResp := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/reconcile/2026-11-01/diff", "", token)
	if csvResp.Code != http.StatusOK || !strings.Contains(csvResp.Body.String(), "alipay") || !strings.Contains(csvResp.Body.String(), "5000") {
		t.Fatalf("差异 CSV 异常: %d %s", csvResp.Code, csvResp.Body.String())
	}
	for _, path := range []string{
		"/api/v1/admin/reconcile/2026-13-01",
		"/api/v1/admin/reconcile/2026-13-01/diff",
		"/api/v1/admin/reconcile?channel=bad",
		"/api/v1/admin/reconcile?limit=0",
	} {
		if got := performJSONWithBearer(t, r, http.MethodGet, path, "", token); got.Code != http.StatusBadRequest {
			t.Fatalf("非法请求 %s 应 400，实际 %d", path, got.Code)
		}
	}

	channels := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/payments/channels", "", token)
	if channels.Code != http.StatusOK || !containsAll(channels.Body.String(), `"channel":"wechat"`, `"configured"`) {
		t.Fatalf("渠道状态异常: %d %s", channels.Code, channels.Body.String())
	}
	if strings.Contains(channels.Body.String(), "api_key") || strings.Contains(channels.Body.String(), "SECRET") {
		t.Fatalf("渠道状态不得返回值: %s", channels.Body.String())
	}
	server.cfg.WXPayMchID = "1900001234"
	server.cfg.WXPayAppID = "wxappid"
	configured := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/payments/channels", "", token)
	if configured.Code != http.StatusOK || !containsAll(configured.Body.String(), `"merchant_last4":"1234"`) {
		t.Fatalf("商户号后四位异常: %d %s", configured.Code, configured.Body.String())
	}
}
