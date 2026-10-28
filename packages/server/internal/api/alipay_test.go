package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/cw-tc/server/internal/identity"
	"github.com/cw-tc/server/internal/platform/db"
	"github.com/cw-tc/server/migrations"
	"github.com/gin-gonic/gin"
)

func TestAlipayHandlersFailClosedWithoutConfiguration(t *testing.T) {
	database, err := db.Open(context.Background(), db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := db.ApplyMigrations(context.Background(), database, migrations.FS, db.DialectSQLite); err != nil {
		t.Fatal(err)
	}
	server := NewWithDatabase(testConfig(t, true), database)
	r := gin.New()
	r.POST("/orders/:orderNo/alipay/precreate", func(c *gin.Context) {
		c.Set(studentContextKey, identity.Student{ID: 1})
		server.alipayPrecreate(c)
	})
	r.POST("/pay/alipay/notify", server.alipayNotify)

	if got := performJSON(t, r, http.MethodPost, "/orders/order-1/alipay/precreate", ""); got.Code != http.StatusServiceUnavailable {
		t.Fatalf("支付宝未配置时预下单必须 fail-closed: %d %s", got.Code, got.Body.String())
	}
	for _, body := range []string{"not=form", ""} {
		w := performJSON(t, r, http.MethodPost, "/pay/alipay/notify", body)
		if w.Code != http.StatusServiceUnavailable {
			t.Fatalf("支付宝未配置时回调必须 fail-closed: %d %s", w.Code, w.Body.String())
		}
	}
}

func TestAlipayNotifyRejectsInvalidAndIgnoresNonSuccess(t *testing.T) {
	server, _ := learningFixture(t)
	server.cfg.AlipayAppID = "app"
	server.cfg.AlipayPrivKey = "private.pem"
	server.cfg.AlipayPubKey = "missing-public.pem"
	server.cfg.AlipayNotify = "https://example.test/notify"
	server.cfg.AlipayGateway = "https://example.test/gateway"
	r := gin.New()
	r.POST("/pay/alipay/notify", server.alipayNotify)
	for _, tc := range []struct {
		body string
		want int
	}{
		{"not=form", http.StatusBadRequest},
		{"trade_status=WAIT_BUYER_PAY", http.StatusBadRequest},
		{"trade_status=TRADE_CLOSED&sign_type=RSA2&app_id=app", http.StatusBadRequest},
	} {
		if got := performJSON(t, r, http.MethodPost, "/pay/alipay/notify", tc.body); got.Code != tc.want {
			t.Errorf("回调错误状态码: %q => %d，期望 %d", tc.body, got.Code, tc.want)
		}
	}
}
