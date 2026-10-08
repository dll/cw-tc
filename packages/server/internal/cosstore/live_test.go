//go:build coslive

// 手动运行的 COS 真机自检（不参与 CI，避免无凭据环境拉低覆盖）。
//
//	在服务器上（已 source /opt/cw-tc/shared/.env）运行：
//	 go test -tags coslive -run TestCosLiveRoundTrip ./internal/cosstore
package cosstore

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func TestCosLiveRoundTrip(t *testing.T) {
	store := New(os.Getenv("CWTC_COS_SECRET_ID"), os.Getenv("CWTC_COS_SECRET_KEY"), os.Getenv("CWTC_COS_BUCKET"), os.Getenv("CWTC_COS_REGION"))
	if store == nil || !store.Enabled() {
		t.Skip("COS 未配置，跳过真机自检")
	}
	ctx := context.Background()
	key := "costest/hello.txt"
	content := "cw-tc cos self-test " + time.Now().Format(time.RFC3339)

	// 读权限诊断：HEAD 一个不存在的对象，403=无访问、404=有读权限。
	if err := store.Head(ctx, key); err != nil {
		t.Logf("⚠ HEAD 诊断: %v", err)
	} else {
		t.Log("✓ HEAD 读权限可用")
	}

	if err := store.Upload(ctx, key, strings.NewReader(content)); err != nil {
		t.Fatalf("上传失败: %v", err)
	}
	t.Logf("✓ 上传成功: %s", key)

	u, err := store.PresignGet(ctx, key, 10*time.Minute)
	if err != nil {
		t.Fatalf("预签名失败: %v", err)
	}
	t.Logf("✓ 预签名 URL: %s", u)

	resp, err := http.Get(u)
	if err != nil {
		t.Fatalf("下载失败: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || string(body) != content {
		t.Fatalf("下载核对失败: status=%d body=%q", resp.StatusCode, body)
	}
	t.Logf("✓ 下载内容一致: %s", content)
	t.Log("COS 真机自检通过")
}
