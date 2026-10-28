package api

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

func TestAlipayNotifyValidSignatureReachesPaymentValidation(t *testing.T) {
	server, _ := learningFixture(t)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	publicDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	publicPath := filepath.Join(t.TempDir(), "alipay-public.pem")
	if err := os.WriteFile(publicPath, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER}), 0600); err != nil {
		t.Fatal(err)
	}
	server.cfg.AlipayAppID = "app"
	server.cfg.AlipayPubKey = publicPath
	server.cfg.AlipayPrivKey = "unused"
	server.cfg.AlipayNotify = "https://example.test/notify"
	server.cfg.AlipayGateway = "https://example.test/gateway"
	r := gin.New()
	r.POST("/pay/alipay/notify", server.alipayNotify)

	for _, fields := range []url.Values{
		{"app_id": {"app"}, "trade_status": {"TRADE_SUCCESS"}, "out_trade_no": {"O-UNKNOWN"}, "trade_no": {"T-1"}, "total_amount": {"bad"}},
		{"app_id": {"app"}, "trade_status": {"TRADE_SUCCESS"}, "out_trade_no": {"O-UNKNOWN"}, "trade_no": {"T-1"}, "total_amount": {"1.00"}},
		{"app_id": {"app"}, "trade_status": {"TRADE_SUCCESS"}, "out_trade_no": {""}, "trade_no": {"T-1"}, "total_amount": {"1.00"}},
	} {
		fields.Set("sign_type", "RSA2")
		fields.Set("sign", signAlipayNotify(t, key, fields))
		got := performJSON(t, r, http.MethodPost, "/pay/alipay/notify", fields.Encode())
		if got.Code != http.StatusBadRequest {
			t.Errorf("未知订单或非法金额应拒绝，实际 %d %s", got.Code, got.Body.String())
		}
	}
}

func signAlipayNotify(t *testing.T, key *rsa.PrivateKey, form url.Values) string {
	t.Helper()
	keys := make([]string, 0, len(form))
	for name, values := range form {
		if name != "sign" && name != "sign_type" && len(values) > 0 && values[0] != "" {
			keys = append(keys, name)
		}
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, name := range keys {
		parts = append(parts, name+"="+form.Get(name))
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "&")))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(signature)
}

func TestAlipayPrecreateRejectsGatewayFailureAfterOrderChecks(t *testing.T) {
	server, _ := learningFixture(t)
	server.cfg.AlipayAppID = "app"
	server.cfg.AlipayPrivKey = "missing-private.pem"
	server.cfg.AlipayPubKey = "missing-public.pem"
	server.cfg.AlipayNotify = "https://example.test/notify"
	server.cfg.AlipayGateway = "https://example.test/gateway"
	if _, err := server.dbConn.Exec(`INSERT INTO orders(order_no,student_id,course_id,course_name_snapshot,price_snapshot_cents,amount_cents,status,expired_at) VALUES('O-ALIPAY',1,1,'课程',1000,1000,'pending','2099-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	r.POST("/orders/:orderNo/alipay/precreate", func(c *gin.Context) {
		c.Set(studentContextKey, identity.Student{ID: 1})
		server.alipayPrecreate(c)
	})
	got := performJSON(t, r, http.MethodPost, "/orders/O-ALIPAY/alipay/precreate", "")
	if got.Code != http.StatusBadGateway {
		t.Fatalf("支付宝网关失败应返回 502: %d %s", got.Code, got.Body.String())
	}
}
