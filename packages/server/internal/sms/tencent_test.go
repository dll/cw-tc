package sms

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTencentSendSuccessAndError(t *testing.T) {
	var action, payload string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		action = r.Header.Get("X-TC-Action")
		if !strings.HasPrefix(r.Header.Get("Authorization"), "TC3-HMAC-SHA256 ") {
			t.Fatalf("缺少 TC3 签名: %s", r.Header.Get("Authorization"))
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		payload = body["TemplateParamSet"].([]any)[0].(string)
		_ = json.NewEncoder(w).Encode(map[string]any{"Response": map[string]any{
			"SendStatusSet": []map[string]string{{"Code": "Ok"}},
		}})
	}))
	defer server.Close()
	sender := &Tencent{SecretID: "id", SecretKey: "key", SdkAppID: "1400", SignName: "签名", TemplateID: "1", Endpoint: server.URL}
	if err := sender.Send(context.Background(), "13812345678", "123456"); err != nil {
		t.Fatal(err)
	}
	if action != "SendSms" || payload != "123456" {
		t.Fatalf("请求内容异常: %s %s", action, payload)
	}

	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"Response": map[string]any{
			"Error": map[string]string{"Code": "AuthFailure", "Message": "bad"},
		}})
	}))
	defer bad.Close()
	sender.Endpoint = bad.URL
	if err := sender.Send(context.Background(), "13812345678", "123456"); err == nil {
		t.Fatal("渠道错误应返回 error")
	}
	if err := (&Tencent{}).Send(context.Background(), "13812345678", "123456"); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("未配置应拒绝: %v", err)
	}
}
