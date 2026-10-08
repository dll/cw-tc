package cosstore

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestBucketEndpoint(t *testing.T) {
	cases := []struct {
		bucket, region, want string
	}{
		{"cwtc-content-1255944979", "ap-nanjing", "https://cwtc-content-1255944979.cos.ap-nanjing.myqcloud.com"},
		{"cwtc-content-1255944979.cos.ap-nanjing.myqcloud.com", "", "https://cwtc-content-1255944979.cos.ap-nanjing.myqcloud.com"},
		{"https://cwtc-content-1255944979.cos.ap-nanjing.myqcloud.com", "", "https://cwtc-content-1255944979.cos.ap-nanjing.myqcloud.com"},
		{"cwtc-content-1255944979", "", ""}, // 桶名 + 缺 region
		{"", "ap-nanjing", ""},              // 缺桶名
	}
	for _, c := range cases {
		if got := bucketEndpoint(c.bucket, c.region); got != c.want {
			t.Errorf("bucketEndpoint(%q,%q)=%q, want %q", c.bucket, c.region, got, c.want)
		}
	}
}

func TestNewFailClosed(t *testing.T) {
	if New("", "k", "b", "r") != nil {
		t.Fatal("缺 secretID 应返回 nil")
	}
	if New("id", "", "b", "r") != nil {
		t.Fatal("缺 secretKey 应返回 nil")
	}
	if New("id", "k", "", "") != nil {
		t.Fatal("缺桶名应返回 nil")
	}
	if New("id", "k", "b", "") != nil {
		t.Fatal("桶名缺 region 应返回 nil")
	}
	if New("id", "k", "b.cos.ap-nanjing.myqcloud.com", "") == nil {
		t.Fatal("端点形式应成功构造")
	}
	if New("id", "k", "b", "ap-nanjing") == nil {
		t.Fatal("桶名+region 应成功构造")
	}
}

func TestStoreEnabled(t *testing.T) {
	if (*Store)(nil).Enabled() {
		t.Fatal("nil Store 应不可用")
	}
	if got := New("id", "k", "b", "ap-nanjing"); got == nil || !got.Enabled() {
		t.Fatal("配置齐全应可用")
	}
}

func TestPresignGetBuildsSignedURL(t *testing.T) {
	store := New("AKIDtest", "testsecretkey", "b.cos.ap-nanjing.myqcloud.com", "")
	if store == nil {
		t.Fatal("构造失败")
	}
	u, err := store.PresignGet(context.Background(), "a/b.mp4", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(u, "q-signature=") || !strings.Contains(u, "b.mp4") {
		t.Fatalf("预签名 URL 格式异常: %s", u)
	}
}
