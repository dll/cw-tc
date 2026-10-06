package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cw-tc/server/internal/identity"
	"github.com/cw-tc/server/internal/platform/db"
	"github.com/cw-tc/server/migrations"
)

func TestLoginEndpointRejectsMalformedJSON(t *testing.T) {
	r := New(testConfig(t, true)).Router()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader("{"))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	r.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"code":10002`) {
		t.Fatalf("非法登录 JSON 响应 = %d %s", response.Code, response.Body.String())
	}
}

func TestLoginEndpointFailsClosedWithoutIdentityService(t *testing.T) {
	r := New(testConfig(t, true)).Router()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"phone":"13812345678","password":"password"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	r.ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("未配置登录服务状态码 = %d", response.Code)
	}
}

func TestLoginEndpointRejectsWrongPasswordWithoutCookie(t *testing.T) {
	database, err := db.Open(context.Background(), db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := db.ApplyMigrations(context.Background(), database, migrations.FS, db.DialectSQLite); err != nil {
		t.Fatal(err)
	}
	service := identity.NewService(database, []byte("01234567890123456789012345678901"), []byte("hmac-key-for-cwTC-tests-012345678901234"))
	if _, err := service.Register(context.Background(), identity.RegisterInput{Name: "样例学员", Phone: "13812345678", IDCard: "110101199001011237", Password: "password-1"}); err != nil {
		t.Fatal(err)
	}
	r := NewWithIdentity(testConfig(t, true), service).Router()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"phone":"13812345678","password":"wrong"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	r.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized || response.Header().Get("Set-Cookie") != "" {
		t.Fatalf("错误密码响应 = %d, cookie=%q", response.Code, response.Header().Get("Set-Cookie"))
	}
}
