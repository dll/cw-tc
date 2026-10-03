package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/cw-tc/server/internal/config"
	"github.com/cw-tc/server/internal/redact"
)

func testConfig(t *testing.T, publicStatus bool) *config.Config {
	t.Helper()
	env := map[string]string{
		config.EnvIDCardEncKey:  strings.Repeat("a", config.IDCardEncKeyBytes),
		config.EnvIDCardHMACKey: strings.Repeat("b", config.MinHMACKeyBytes),
		config.EnvSessionSecret: strings.Repeat("c", config.MinHMACKeyBytes),
	}
	if !publicStatus {
		env[config.EnvPublicStatusEnabled] = "false"
	}
	cfg, err := config.LoadFrom(func(k string) string { return env[k] })
	if err != nil {
		t.Fatalf("测试配置构造失败: %v", err)
	}
	return cfg
}

func newTestRouter(t *testing.T, publicStatus bool) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	return New(testConfig(t, publicStatus)).Router()
}

func doGET(t *testing.T, r *gin.Engine, path string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	r.ServeHTTP(w, req)

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("响应不是合法 JSON: %v\nbody=%s", err, w.Body.String())
	}
	return w, body
}

func TestHealth(t *testing.T) {
	r := newTestRouter(t, true)
	w, body := doGET(t, r, "/api/v1/health")

	if w.Code != http.StatusOK {
		t.Fatalf("健康检查应返回 200，实际 %d", w.Code)
	}
	if code, _ := body["code"].(float64); code != 0 {
		t.Errorf("信封 code 应为 0，实际 %v", body["code"])
	}
	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("data 字段缺失或类型错误: %v", body["data"])
	}
	if data["status"] != "ok" {
		t.Errorf("status 应为 ok，实际 %v", data["status"])
	}
	if _, ok := data["version"]; !ok {
		t.Error("健康检查应返回 version")
	}
}

// TestPublicStatus_ShapeAndNoPersonalData
// 验证 PRD v2 §7.12 K-2 的硬约束：只读、聚合、**不含任何个人信息**。
func TestPublicStatus_ShapeAndNoPersonalData(t *testing.T) {
	r := newTestRouter(t, true)
	w, body := doGET(t, r, "/api/v1/public/status")

	if w.Code != http.StatusOK {
		t.Fatalf("应返回 200，实际 %d", w.Code)
	}
	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("data 缺失: %v", body)
	}
	if data["source"] != "cw-tc" {
		t.Errorf("source 应为 cw-tc，实际 %v", data["source"])
	}

	// 计数尚无条件接入数据源 -> 必须是 null，不能是 0。
	// 返回 0 会被读成"真的没有课程"，属于铁律二禁止的编数。
	courses, _ := data["courses"].(map[string]any)
	if courses == nil {
		t.Fatal("courses 字段缺失")
	}
	for _, k := range []string{"total", "published", "byMajor"} {
		v, exists := courses[k]
		if !exists {
			t.Errorf("courses.%s 字段缺失", k)
			continue
		}
		if v != nil {
			t.Errorf("courses.%s 在接入数据源前必须为 null（不得编数），实际 %v", k, v)
		}
	}
	if data["dataSource"] != "not_wired" {
		t.Errorf("应显式标注数据源未接入，实际 %v", data["dataSource"])
	}

	// 反向检查：响应体里不得出现任何个人信息字段名或敏感值模式
	raw := w.Body.String()
	for _, forbidden := range []string{"idCard", "id_card", "phone", "name", "studentNo", "orderNo"} {
		if strings.Contains(raw, forbidden) {
			t.Errorf("公开状态响应不得包含字段 %q，实际响应: %s", forbidden, raw)
		}
	}
	if redact.ContainsIDCard(raw) {
		t.Errorf("公开状态响应含身份证号: %s", raw)
	}
	if redact.ContainsPhone(raw) {
		t.Errorf("公开状态响应含手机号: %s", raw)
	}
}

// TestPublicStatus_Disabled 关闭时路由不注册（避免"存在但不可用"的暧昧状态）。
func TestPublicStatus_Disabled(t *testing.T) {
	r := newTestRouter(t, false)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/public/status", nil))
	if w.Code != http.StatusNotFound {
		t.Errorf("关闭后应 404，实际 %d", w.Code)
	}
}

// TestAccessLog_RedactsQuery 验证访问日志对 URL 的兜底过滤。
// 即便有人把身份证号拼进 query（PRD v2 §9.2 明令禁止），日志里也不能留明文。
func TestAccessLog_RedactsQuery(t *testing.T) {
	cfg := testConfig(t, true)
	if !cfg.LogRedactEnabled {
		t.Fatal("本用例前提是脱敏开关为默认开启")
	}
	raw := "/api/v1/health?id=110101199001011234&phone=13812345678"
	got := redact.String(raw)

	if redact.ContainsIDCard(got) || redact.ContainsPhone(got) {
		t.Errorf("访问日志兜底过滤失效: %q", got)
	}
	if strings.Contains(got, "110101199001011234") || strings.Contains(got, "13812345678") {
		t.Errorf("访问日志仍含明文: %q", got)
	}
}
