package api

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/cw-tc/server/internal/identity"
	"github.com/cw-tc/server/internal/platform/db"
	"github.com/cw-tc/server/migrations"
)

func TestCompleteIdentityEndpoint(t *testing.T) {
	database, err := db.Open(context.Background(), db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := db.ApplyMigrations(context.Background(), database, migrations.FS, db.DialectSQLite); err != nil {
		t.Fatal(err)
	}
	service := identity.NewService(database, []byte("01234567890123456789012345678901"), []byte("hmac-key-for-cwTC-tests-012345678901234"))
	student, err := service.Register(context.Background(), identity.RegisterInput{
		Name: "样例学员", Phone: "13812345678", Password: "password-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	session, err := service.CreateSession(context.Background(), student.ID, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	r := NewWithIdentityAndDatabase(testConfig(t, true), service, database).Router()

	before := performJSONWithCookie(t, r, http.MethodGet, "/api/v1/me", ``, session.Token)
	if before.Code != http.StatusOK || !containsAll(before.Body.String(),
		`"identity_verified":false`, `"identity_masked":""`) {
		t.Fatalf("未实名状态异常: %d %s", before.Code, before.Body.String())
	}
	if bad := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/me/identity", `not-json`, session.Token); !strings.Contains(bad.Body.String(), `"code":10002`) {
		t.Fatalf("非法 JSON 应拒绝: %d %s", bad.Code, bad.Body.String())
	}
	if invalid := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/me/identity", `{"id_card":"bad"}`, session.Token); !strings.Contains(invalid.Body.String(), `"code":10002`) {
		t.Fatalf("非法身份证应拒绝: %d %s", invalid.Code, invalid.Body.String())
	}

	got := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/me/identity",
		`{"id_card":"110101199001011237"}`, session.Token)
	if got.Code != http.StatusOK || !containsAll(got.Body.String(),
		`"identity_verified":true`, `"identity_masked":"110101********1237"`) {
		t.Fatalf("补充实名失败: %d %s", got.Code, got.Body.String())
	}

	me := performJSONWithCookie(t, r, http.MethodGet, "/api/v1/me", ``, session.Token)
	if me.Code != http.StatusOK || !containsAll(me.Body.String(),
		`"identity_verified":true`, `"identity_masked":"110101********1237"`) {
		t.Fatalf("/me 未返回脱敏实名状态: %d %s", me.Code, me.Body.String())
	}
	if strings.Contains(me.Body.String(), "110101199001011237") {
		t.Fatal("/me 不得返回身份证明文")
	}

	again := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/me/identity",
		`{"id_card":"110101199003076755"}`, session.Token)
	if again.Code != http.StatusOK || !strings.Contains(again.Body.String(), `"code":10002`) {
		t.Fatalf("重复补充实名应拒绝: %d %s", again.Code, again.Body.String())
	}
}
