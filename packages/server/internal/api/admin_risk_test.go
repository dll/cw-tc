package api

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/cw-tc/server/internal/risk"
)

func TestAdminRiskBlacklistLifecycle(t *testing.T) {
	server, database, token, _ := adminDetailServer(t)
	defer database.Close()
	r := server.Router()
	until := time.Now().UTC().Add(30 * 24 * time.Hour).Format(time.RFC3339)
	for _, body := range []string{
		`{"target_type":"phone","target_value":"13812345678","reason":"恶意退款","expires_at":"` + until + `","confirm":false}`,
		`{"target_type":"bad","target_value":"x","reason":"x","expires_at":"` + until + `","confirm":true}`,
		`{"target_type":"phone","target_value":"13812345678","reason":"x","expires_at":"2000-01-01T00:00:00Z","confirm":true}`,
		`{"target_type":"phone","target_value":"","reason":"x","expires_at":"` + until + `","confirm":true}`,
	} {
		if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/risk/blacklist", body, token); got.Code != http.StatusBadRequest {
			t.Fatalf("非法黑名单应 400: %s -> %d", body, got.Code)
		}
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/risk/blacklist",
		`{"target_type":"phone","target_value":"13812345678","reason":"恶意退款","expires_at":"`+until+`","confirm":true}`, token); got.Code != http.StatusOK {
		t.Fatalf("加入黑名单失败: %d %s", got.Code, got.Body.String())
	}
	list := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/risk?status=active&target_type=phone", "", token)
	if list.Code != http.StatusOK || !containsAll(list.Body.String(), `"target_value_masked":"138****5678"`, `"refund_abuse_limit":3`) {
		t.Fatalf("黑名单列表异常: %d %s", list.Code, list.Body.String())
	}
	if strings.Contains(list.Body.String(), "13812345678") {
		t.Fatalf("黑名单不得返回明文手机号: %s", list.Body.String())
	}
	for _, query := range []string{"?status=bad", "?target_type=bad", "?limit=0"} {
		if got := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/risk"+query, "", token); got.Code != http.StatusBadRequest {
			t.Fatalf("非法筛选 %s 应 400，实际 %d", query, got.Code)
		}
	}

	var id int64
	if err := database.QueryRow(`SELECT id FROM risk_blacklist WHERE status='active'`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if err := risk.Appeal(context.Background(), database, id, "并非本人操作，申请复核"); err != nil {
		t.Fatal(err)
	}
	if got := performJSONWithBearer(t, r, http.MethodPatch, "/api/v1/admin/risk/blacklist/"+itoa64(id),
		`{"accept":true,"note":"核实误伤，解除","confirm":false}`, token); got.Code != http.StatusBadRequest {
		t.Fatalf("缺少二次确认应 400，实际 %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodPatch, "/api/v1/admin/risk/blacklist/"+itoa64(id),
		`{"accept":true,"note":"核实误伤，解除","confirm":true}`, token); got.Code != http.StatusOK {
		t.Fatalf("处理申诉失败: %d %s", got.Code, got.Body.String())
	}
	var status string
	if err := database.QueryRow(`SELECT status FROM risk_blacklist WHERE id=$1`, id).Scan(&status); err != nil || status != "expired" {
		t.Fatalf("受理申诉应解除黑名单: %s %v", status, err)
	}
	if again := performJSONWithBearer(t, r, http.MethodPatch, "/api/v1/admin/risk/blacklist/"+itoa64(id),
		`{"accept":true,"note":"重复","confirm":true}`, token); again.Code != http.StatusConflict {
		t.Fatalf("重复处理应 409，实际 %d", again.Code)
	}
	var audits int
	if err := database.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action IN ('risk.blacklist_add','risk.blacklist_appeal')`).Scan(&audits); err != nil || audits != 2 {
		t.Fatalf("风控操作必须写审计: %d %v", audits, err)
	}
}
