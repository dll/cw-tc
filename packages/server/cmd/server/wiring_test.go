package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cw-tc/server/internal/config"
)

func TestBuildAPIWiresIdentityServiceToProductionRouter(t *testing.T) {
	cfg := &config.Config{
		IDCardEncKey:        strings.Repeat("a", config.IDCardEncKeyBytes),
		IDCardHMACKey:       strings.Repeat("b", config.MinHMACKeyBytes),
		SessionSecret:       strings.Repeat("c", config.MinHMACKeyBytes),
		DBDriver:            "sqlite",
		DeployEnv:           "development",
		PublicStatusEnabled: true,
		LogRedactEnabled:    true,
	}
	database, closeDB, err := openDatabase(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer closeDB()
	r := buildAPI(cfg, database).Router()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"name":"样例学员","phone":"13812345678","id_card":"110101199001011237","password":"password-1"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	r.ServeHTTP(response, request)
	if response.Code == http.StatusServiceUnavailable {
		t.Fatalf("正式服务路由未接入账号服务: %s", response.Body.String())
	}
}
