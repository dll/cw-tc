package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cw-tc/server/internal/identity"
	"github.com/cw-tc/server/internal/platform/db"
	"github.com/cw-tc/server/migrations"
)

func TestMeEndpointRequiresSessionAndReturnsWhitelist(t *testing.T) {
	database, err := db.Open(context.Background(), db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := db.ApplyMigrations(context.Background(), database, migrations.FS, db.DialectSQLite); err != nil {
		t.Fatal(err)
	}
	service := identity.NewService(database, []byte("01234567890123456789012345678901"), []byte("hmac-key-for-cwTC-tests-012345678901234"))
	student, err := service.Register(context.Background(), identity.RegisterInput{Name: "样例学员", Phone: "13812345678", IDCard: "110101199001011237", Password: "password-1"})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	session, err := service.CreateSession(context.Background(), student.ID, now)
	if err != nil {
		t.Fatal(err)
	}
	r := NewWithIdentity(testConfig(t, true), service).Router()
	for _, tc := range []struct {
		name  string
		value string
		want  int
	}{
		{"anonymous", "", http.StatusUnauthorized},
		{"cookie", "cookie:" + session.Token, http.StatusOK},
		{"bearer", "bearer:" + session.Token, http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
			if strings.HasPrefix(tc.value, "cookie:") {
				request.AddCookie(&http.Cookie{Name: "cw_tc_sid", Value: strings.TrimPrefix(tc.value, "cookie:")})
			}
			if strings.HasPrefix(tc.value, "bearer:") {
				request.Header.Set("Authorization", "Bearer "+strings.TrimPrefix(tc.value, "bearer:"))
			}
			response := httptest.NewRecorder()
			r.ServeHTTP(response, request)
			if response.Code != tc.want {
				t.Fatalf("状态码 = %d, body=%s", response.Code, response.Body.String())
			}
			if tc.want == http.StatusOK && strings.Contains(response.Body.String(), "id_card") {
				t.Fatal("/me 不得返回身份证字段")
			}
		})
	}
}
