package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cw-tc/server/internal/identity"
	"github.com/cw-tc/server/internal/platform/db"
	"github.com/cw-tc/server/migrations"
)

func TestLoginEndpointSetsHttpOnlySessionCookie(t *testing.T) {
	database, err := db.Open(context.Background(), db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := db.ApplyMigrations(context.Background(), database, migrations.FS, db.DialectSQLite); err != nil {
		t.Fatal(err)
	}
	service := identity.NewService(database, []byte("01234567890123456789012345678901"), []byte("hmac-key-for-cwtc-tests-012345678901234"))
	if _, err := service.Register(context.Background(), identity.RegisterInput{Name: "样例学员", Phone: "13812345678", IDCard: "110101199001011237", Password: "password-1"}); err != nil {
		t.Fatal(err)
	}
	server := NewWithIdentity(testConfig(t, true), service)
	r := server.Router()
	body := `{"phone":"13812345678","password":"password-1"}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	r.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("登录状态码 = %d, body=%s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Header().Get("Set-Cookie"), "cw_tc_sid=") || !strings.Contains(response.Header().Get("Set-Cookie"), "HttpOnly") {
		t.Fatalf("登录必须设置 HttpOnly 会话 Cookie: %s", response.Header().Get("Set-Cookie"))
	}
	var envelope map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(response.Body.String(), "password") {
		t.Fatal("登录响应不得返回密码")
	}
}
