package api

import (
	"net/http"
	"testing"
)

func TestPaymentCallbackRejectsMalformedAndUnconfiguredRequests(t *testing.T) {
	r := New(testConfig(t, true)).Router()
	malformed := performJSON(t, r, http.MethodPost, "/api/v1/payment/callback", `{}`)
	if malformed.Code != http.StatusServiceUnavailable || malformed.Body.String() == "" {
		t.Fatalf("未配置支付通道应 fail-closed: %d %s", malformed.Code, malformed.Body.String())
	}
}
