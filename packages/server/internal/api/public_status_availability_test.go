package api

import (
	"net/http"
	"testing"
)

func TestPublicStatusIsUnavailableBeforeDataSourceWiring(t *testing.T) {
	r := newTestRouter(t, true)
	w, body := doGET(t, r, "/api/v1/public/status")
	if w.Code != http.StatusOK {
		t.Fatalf("状态接口应返回 200 信封，实际 %d", w.Code)
	}
	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("data 字段缺失: %v", body)
	}
	if data["status"] != "unavailable" {
		t.Fatalf("数据源未接入时不得伪造 available，实际 %v", data["status"])
	}
	if data["dataSource"] != "not_wired" {
		t.Fatalf("应明确标注数据源未接入，实际 %v", data["dataSource"])
	}
}
