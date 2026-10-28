package cosstore

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/tencentyun/cos-go-sdk-v5"
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

func TestUploadAndHeadRequestPaths(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			if _, err := io.ReadAll(r.Body); err != nil {
				t.Errorf("读取上传内容失败: %v", err)
			}
			w.WriteHeader(http.StatusInternalServerError)
		case http.MethodHead:
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	client := cos.NewClient(&cos.BaseURL{BucketURL: u}, srv.Client())
	store := &Store{client: client, secretID: "id", secretKey: "key"}
	if err := store.Upload(context.Background(), "lesson/video.mp4", strings.NewReader("data")); err == nil {
		t.Fatal("上传返回 500 时应报告错误")
	}
	if err := store.Head(context.Background(), "lesson/video.mp4"); err == nil {
		t.Fatal("HEAD 返回 500 时应报告错误")
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

func TestProbeURL(t *testing.T) {
	// 206 → 可读
	srv206 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") != "bytes=0-0" {
			t.Errorf("应带 Range 头，实际 %q", r.Header.Get("Range"))
		}
		w.Header().Set("Content-Range", "bytes 0-0/1024")
		w.WriteHeader(http.StatusPartialContent)
	}))
	defer srv206.Close()
	if !probeURL(context.Background(), srv206.URL, srv206.Client()) {
		t.Fatal("206 应返回 true")
	}
	// 451（COS 欠费/停服）→ 不可读
	srv451 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnavailableForLegalReasons)
	}))
	defer srv451.Close()
	if probeURL(context.Background(), srv451.URL, srv451.Client()) {
		t.Fatal("451 应返回 false")
	}
	// 连接失败 → false（指向已关闭的 server）
	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	deadURL := dead.URL
	dead.Close()
	if probeURL(context.Background(), deadURL, dead.Client()) {
		t.Fatal("连接失败应返回 false")
	}
}
