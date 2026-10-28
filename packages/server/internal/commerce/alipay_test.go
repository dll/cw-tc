package commerce

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func alipayKeyFiles(t *testing.T) (*rsa.PrivateKey, string, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	privatePath := filepath.Join(dir, "private.pem")
	privateDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(privatePath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER}), 0600); err != nil {
		t.Fatal(err)
	}
	publicPath := filepath.Join(dir, "public.pem")
	publicDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(publicPath, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER}), 0600); err != nil {
		t.Fatal(err)
	}
	return key, privatePath, publicPath
}

func signedResponse(t *testing.T, key *rsa.PrivateKey, outTradeNo, qrCode, code string) []byte {
	t.Helper()
	response, err := json.Marshal(map[string]any{"code": code, "msg": "ok", "out_trade_no": outTradeNo, "qr_code": qrCode})
	if err != nil {
		t.Fatal(err)
	}
	// 响应签名按支付宝协议直接签 response JSON 对象。
	sum := sha256.Sum256(response)
	raw, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatal(err)
	}
	return []byte(`{"alipay_trade_precreate_response":` + string(response) + `,"sign":"` + base64.StdEncoding.EncodeToString(raw) + `"}`)
}

func TestAlipayPrecreateSignsRequestAndVerifiesResponse(t *testing.T) {
	key, privatePath, publicPath := alipayKeyFiles(t)
	var gotForm url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || !strings.Contains(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
			t.Errorf("请求格式错误: %s %s", r.Method, r.Header.Get("Content-Type"))
		}
		if err := r.ParseForm(); err != nil {
			t.Errorf("解析请求失败: %v", err)
			return
		}
		gotForm = r.PostForm
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(signedResponse(t, key, "ORD-1", "https://qr.example/1", "10000"))
	}))
	defer server.Close()
	client := AlipayClient{AppID: "app-1", PrivateKeyPath: privatePath, PublicKeyPath: publicPath, NotifyURL: "https://example/notify", Gateway: server.URL}
	result, err := client.Precreate(context.Background(), "ORD-1", "样例课程", 1234)
	if err != nil {
		t.Fatal(err)
	}
	if result.QRCode != "https://qr.example/1" || result.TradeNo != "ORD-1" {
		t.Fatalf("下单结果错误: %+v", result)
	}
	if gotForm.Get("app_id") != "app-1" || gotForm.Get("method") != "alipay.trade.precreate" || gotForm.Get("sign") == "" {
		t.Fatalf("请求参数缺少签名或业务字段: %v", gotForm)
	}
	var biz map[string]string
	if err := json.Unmarshal([]byte(gotForm.Get("biz_content")), &biz); err != nil || biz["total_amount"] != "12.34" || biz["out_trade_no"] != "ORD-1" {
		t.Fatalf("金额或订单参数错误: %v, %v", biz, err)
	}
	if err := client.VerifyNotify(url.Values{"sign_type": {"RSA2"}, "app_id": {"app-1"}, "trade_status": {"TRADE_SUCCESS"}, "out_trade_no": {"ORD-1"}, "sign": {"bad"}}); err == nil {
		t.Fatal("无效通知签名应拒绝")
	}
}

func TestAlipayPrecreateFailsClosed(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		code int
	}{
		{name: "gateway status", body: `failure`, code: http.StatusBadGateway},
		{name: "malformed response", body: `{`, code: http.StatusOK},
		{name: "business error", body: `{"code":"40004","msg":"bad","out_trade_no":"ORD-1","qr_code":""}`, code: http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, privatePath, publicPath := alipayKeyFiles(t)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc.code != http.StatusOK {
					w.WriteHeader(tc.code)
				}
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			client := AlipayClient{AppID: "app", PrivateKeyPath: privatePath, PublicKeyPath: publicPath, NotifyURL: "https://example/notify", Gateway: server.URL}
			if _, err := client.Precreate(context.Background(), "ORD-1", "课程", 100); err == nil {
				t.Fatal("错误响应必须失败")
			}
		})
	}
	client := AlipayClient{}
	if _, err := client.Precreate(context.Background(), "", "", 0); err == nil {
		t.Fatal("缺少配置或订单金额应失败")
	}
	if _, err := loadPrivateKey(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("私钥文件缺失应失败")
	}
	bad := filepath.Join(t.TempDir(), "bad.pem")
	if err := os.WriteFile(bad, []byte("not pem"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadPrivateKey(bad); err == nil {
		t.Fatal("非法私钥格式应失败")
	}
}

func TestAlipayNotifySignatureAndAmountValidation(t *testing.T) {
	key, _, publicPath := alipayKeyFiles(t)
	client := AlipayClient{AppID: "app", PublicKeyPath: publicPath}
	form := url.Values{"app_id": {"app"}, "sign_type": {"RSA2"}, "out_trade_no": {"ORD-1"}, "total_amount": {"1.00"}}
	params := map[string]string{"app_id": "app", "out_trade_no": "ORD-1", "total_amount": "1.00"}
	form.Set("sign", signNotify(t, key, params))
	if err := client.VerifyNotify(form); err != nil {
		t.Fatalf("有效通知签名被拒绝: %v", err)
	}
	form.Set("app_id", "other")
	if err := client.VerifyNotify(form); err == nil {
		t.Fatal("应用 ID 不匹配应拒绝")
	}
	for _, amount := range []string{"1", "1.0", "-1.00", "1.100", "x.00", "1.aa", "92233720368547759.00"} {
		if _, err := AlipayAmountCents(amount); err == nil {
			t.Errorf("非法金额 %q 被接受", amount)
		}
	}
	for amount, want := range map[string]int64{"0.00": 0, "12.34": 1234, "999.99": 99999} {
		got, err := AlipayAmountCents(amount)
		if err != nil || got != want {
			t.Errorf("金额 %q = %d, %v; want %d", amount, got, err, want)
		}
	}
	if err := client.VerifyResponse([]byte(`{}`), "bad"); err == nil {
		t.Fatal("缺少 response 对象应拒绝")
	}
}

func signNotify(t *testing.T, key *rsa.PrivateKey, params map[string]string) string {
	t.Helper()
	sig, err := rsaSign(key, params)
	if err != nil {
		t.Fatal(err)
	}
	return sig
}
