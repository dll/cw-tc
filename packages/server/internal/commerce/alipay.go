package commerce

// 支付宝当面付网关：业务层只拿到订单二维码，签名与渠道请求集中在此处。
// 不保存或返回私钥内容；配置只传文件路径。
import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

type AlipayClient struct {
	AppID, PrivateKeyPath, PublicKeyPath, NotifyURL, Gateway string
	HTTPClient                                               *http.Client
}
type AlipayPrecreateResult struct {
	QRCode  string
	TradeNo string
}

func (c AlipayClient) Precreate(ctx context.Context, orderNo, subject string, amountCents int64) (AlipayPrecreateResult, error) {
	if c.AppID == "" || c.PrivateKeyPath == "" || c.NotifyURL == "" || c.Gateway == "" || orderNo == "" || amountCents <= 0 {
		return AlipayPrecreateResult{}, errors.New("支付宝配置或订单参数不完整")
	}
	priv, err := loadPrivateKey(c.PrivateKeyPath)
	if err != nil {
		return AlipayPrecreateResult{}, err
	}
	params := map[string]string{"app_id": c.AppID, "method": "alipay.trade.precreate", "format": "JSON", "charset": "utf-8", "sign_type": "RSA2", "timestamp": time.Now().Format("2006-01-02 15:04:05"), "version": "1.0", "notify_url": c.NotifyURL, "biz_content": mustJSON(map[string]any{"out_trade_no": orderNo, "total_amount": fmt.Sprintf("%.2f", float64(amountCents)/100), "subject": subject})}
	params["sign"], err = rsaSign(priv, params)
	if err != nil {
		return AlipayPrecreateResult{}, err
	}
	resp, err := c.do(ctx, params)
	if err != nil {
		return AlipayPrecreateResult{}, err
	}
	var envelope struct {
		Response struct {
			Code       string `json:"code"`
			Msg        string `json:"msg"`
			OutTradeNo string `json:"out_trade_no"`
			QRCode     string `json:"qr_code"`
		} `json:"alipay_trade_precreate_response"`
		Sign string `json:"sign"`
	}
	if err := json.Unmarshal(resp, &envelope); err != nil {
		return AlipayPrecreateResult{}, err
	}
	if err := c.VerifyResponse(resp, envelope.Sign); err != nil {
		return AlipayPrecreateResult{}, err
	}
	if envelope.Response.OutTradeNo != orderNo {
		return AlipayPrecreateResult{}, ErrCallbackInvalid
	}
	if envelope.Response.Code != "10000" || envelope.Response.QRCode == "" {
		return AlipayPrecreateResult{}, fmt.Errorf("支付宝下单失败: %s", envelope.Response.Msg)
	}
	return AlipayPrecreateResult{QRCode: envelope.Response.QRCode, TradeNo: envelope.Response.OutTradeNo}, nil
}

func (c AlipayClient) do(ctx context.Context, params map[string]string) ([]byte, error) {
	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Gateway, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=utf-8")
	h := c.HTTPClient
	if h == nil {
		h = http.DefaultClient
	}
	res, err := h.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode/100 != 2 {
		return nil, fmt.Errorf("支付宝网关 HTTP %d", res.StatusCode)
	}
	return io.ReadAll(res.Body)
}

func loadPrivateKey(path string) (*rsa.PrivateKey, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(b)
	if block == nil {
		return nil, errors.New("支付宝私钥格式错误")
	}
	if k, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return k, nil
	}
	k, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	rsaKey, ok := k.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("支付宝私钥不是 RSA")
	}
	return rsaKey, nil
}
func mustJSON(v any) string { b, _ := json.Marshal(v); return string(b) }
func rsaSign(key *rsa.PrivateKey, params map[string]string) (string, error) {
	keys := make([]string, 0, len(params))
	for k, v := range params {
		if k != "sign" && v != "" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte('&')
		}
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(params[k])
	}
	h := sha256.Sum256([]byte(b.String()))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, cryptoHash(), h[:])
	return base64.StdEncoding.EncodeToString(sig), err
}
func cryptoHash() crypto.Hash { return crypto.SHA256 }

func (c AlipayClient) publicKey() (*rsa.PublicKey, error) {
	b, err := os.ReadFile(c.PublicKeyPath)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(b)
	if block == nil {
		return nil, errors.New("支付宝公钥格式错误")
	}
	if k, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
		pub, ok := k.(*rsa.PublicKey)
		if ok {
			return pub, nil
		}
	}
	if pub, err := x509.ParsePKCS1PublicKey(block.Bytes); err == nil {
		return pub, nil
	}
	return nil, errors.New("支付宝公钥不是 RSA")
}
func verifyRSA(pub *rsa.PublicKey, message, signature string) error {
	raw, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return err
	}
	sum := sha256.Sum256([]byte(message))
	return rsa.VerifyPKCS1v15(pub, crypto.SHA256, sum[:], raw)
}
func (c AlipayClient) VerifyNotify(form url.Values) error {
	pub, err := c.publicKey()
	if err != nil {
		return err
	}
	params := map[string]string{}
	for k, v := range form {
		if len(v) > 0 && k != "sign" && k != "sign_type" {
			params[k] = v[0]
		}
	}
	keys := make([]string, 0, len(params))
	for k, v := range params {
		if v != "" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte('&')
		}
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(params[k])
	}
	if form.Get("sign_type") != "RSA2" || form.Get("app_id") != c.AppID {
		return ErrCallbackInvalid
	}
	if err := verifyRSA(pub, b.String(), form.Get("sign")); err != nil {
		return ErrCallbackInvalid
	}
	return nil
}
func (c AlipayClient) VerifyResponse(body []byte, signature string) error {
	pub, err := c.publicKey()
	if err != nil {
		return err
	}
	key := []byte(`"alipay_trade_precreate_response"`)
	start := bytes.Index(body, key)
	if start < 0 {
		return ErrCallbackInvalid
	}
	start += len(key)
	for start < len(body) && (body[start] == ' ' || body[start] == ':' || body[start] == '\n') {
		start++
	}
	if start >= len(body) || body[start] != '{' {
		return ErrCallbackInvalid
	}
	depth := 0
	quoted := false
	escape := false
	end := start
	for ; end < len(body); end++ {
		ch := body[end]
		if escape {
			escape = false
			continue
		}
		if quoted && ch == '\\' {
			escape = true
			continue
		}
		if ch == '"' {
			quoted = !quoted
			continue
		}
		if quoted {
			continue
		}
		if ch == '{' {
			depth++
		}
		if ch == '}' {
			depth--
			if depth == 0 {
				end++
				break
			}
		}
	}
	if depth != 0 {
		return ErrCallbackInvalid
	}
	if err := verifyRSA(pub, string(body[start:end]), signature); err != nil {
		return ErrCallbackInvalid
	}
	return nil
}
func AlipayAmountCents(amount string) (int64, error) {
	parts := strings.Split(amount, ".")
	if len(parts) != 2 || len(parts[1]) != 2 {
		return 0, ErrCallbackInvalid
	}
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || whole < 0 {
		return 0, ErrCallbackInvalid
	}
	cents, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || cents < 0 || cents > 99 || whole > (int64(^uint64(0)>>1)-cents)/100 {
		return 0, ErrCallbackInvalid
	}
	return whole*100 + cents, nil
}
