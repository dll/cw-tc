package resource

import (
	"testing"
	"time"
)

func TestSignVerifyRoundTrip(t *testing.T) {
	secret := []byte("resource-sign-secret-0123456789abcdef")
	key := "07-CN/lectures/10/slides.pdf"
	expires := time.Now().Add(5 * time.Minute)
	sig := Sign(secret, key, expires)
	if !Verify(secret, key, expires, sig) {
		t.Fatal("合法签名应通过校验")
	}
}

func TestVerifyRejectsTamperedOrExpired(t *testing.T) {
	secret := []byte("resource-sign-secret-0123456789abcdef")
	key := "07-CN/lectures/10/slides.pdf"
	expires := time.Now().Add(5 * time.Minute)
	if Verify(secret, key, expires, "deadbeef") {
		t.Fatal("伪造签名应被拒绝")
	}
	if Verify(secret, "other-key", expires, Sign(secret, key, expires)) {
		t.Fatal("篡改存储键应被拒绝")
	}
	if Verify(secret, key, time.Now().Add(-time.Minute), Sign(secret, key, time.Now().Add(-time.Minute))) {
		t.Fatal("过期签名应被拒绝")
	}
	if Verify(nil, key, expires, Sign(secret, key, expires)) {
		t.Fatal("空密钥应被拒绝")
	}
}
