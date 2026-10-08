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

func TestRegisterEndpointReturnsStudentWithoutSensitiveFields(t *testing.T) {
	database, err := db.Open(context.Background(), db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := db.ApplyMigrations(context.Background(), database, migrations.FS, db.DialectSQLite); err != nil {
		t.Fatal(err)
	}
	service := identity.NewService(database, []byte("01234567890123456789012345678901"), []byte("hmac-key-for-cwtc-tests-012345678901234"))
	r := NewWithIdentity(testConfig(t, true), service).Router()
	body := `{"name":"样例学员","phone":"13812345678","id_card":"110101199001011237","password":"password-1"}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	r.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("注册状态码 = %d, body=%s", response.Code, response.Body.String())
	}
	var envelope map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(envelope)
	if strings.Contains(string(raw), "id_card") || strings.Contains(string(raw), "password") {
		t.Fatalf("注册响应不得返回敏感字段: %s", raw)
	}
	// 注册即登录：应返回 token（移动端存安全存储、走 Bearer）
	data, _ := envelope["data"].(map[string]any)
	if data == nil || data["token"] == nil || data["token"] == "" {
		t.Fatalf("注册响应应返回 token: %s", raw)
	}
}

func TestRegisterEndpointDuplicates(t *testing.T) {
	database, err := db.Open(context.Background(), db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := db.ApplyMigrations(context.Background(), database, migrations.FS, db.DialectSQLite); err != nil {
		t.Fatal(err)
	}
	service := identity.NewService(database, []byte("01234567890123456789012345678901"), []byte("hmac-key-for-cwtc-tests-012345678901234"))
	r := NewWithIdentity(testConfig(t, true), service).Router()
	doRegister := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec
	}
	doRegister(`{"name":"样例","phone":"13812345678","id_card":"110101199001011237","password":"password-1"}`)
	// 同手机号 → 10103
	if got := doRegister(`{"name":"样例","phone":"13812345678","id_card":"110101199003076755","password":"password-1"}`); !strings.Contains(got.Body.String(), `"code":10103`) {
		t.Fatalf("同手机号应 10103: %s", got.Body.String())
	}
	// 同身份证号 → 10104
	if got := doRegister(`{"name":"样例","phone":"13812345679","id_card":"110101199001011237","password":"password-1"}`); !strings.Contains(got.Body.String(), `"code":10104`) {
		t.Fatalf("同身份证号应 10104: %s", got.Body.String())
	}
}
