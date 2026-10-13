// Package cosstore 封装腾讯云 COS 私有读写桶的访问（视频/音频资源）。
// 未配置凭据/桶时 New 返回 nil（fail-closed），调用方据此回退文件服务。
package cosstore

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/tencentyun/cos-go-sdk-v5"
)

// Store 是 COS 客户端的最小封装（预签名下载 + 上传）。
type Store struct {
	client    *cos.Client
	secretID  string
	secretKey string
}

// New 创建 COS 客户端。secretID/secretKey/bucket 任一缺失，或桶端点解析不出，
// 返回 nil —— 调用方必须据此把视频/音频下载回退到文件服务，绝不能 panic。
// bucket 兼容两种填法：完整端点（含 .cos./myqcloud.com）或「桶名 + region」。
func New(secretID, secretKey, bucket, region string) *Store {
	endpoint := bucketEndpoint(bucket, region)
	if secretID == "" || secretKey == "" || endpoint == "" {
		return nil
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil
	}
	client := cos.NewClient(&cos.BaseURL{BucketURL: u}, &http.Client{
		Transport: &cos.AuthorizationTransport{
			SecretID:  secretID,
			SecretKey: secretKey,
		},
	})
	return &Store{client: client, secretID: secretID, secretKey: secretKey}
}

// Enabled 报告 Store 是否可用（New 非 nil）。
func (s *Store) Enabled() bool { return s != nil && s.client != nil }

// bucketEndpoint 归一化桶端点，返回 `https://…` 形式的 BucketURL；解析不出返回空串。
func bucketEndpoint(bucket, region string) string {
	b := strings.TrimSpace(bucket)
	b = strings.TrimPrefix(strings.TrimPrefix(b, "https://"), "http://")
	if b == "" {
		return ""
	}
	if strings.Contains(b, ".myqcloud.com") || strings.Contains(b, ".cos.") {
		return "https://" + b
	}
	if strings.TrimSpace(region) == "" {
		return ""
	}
	return "https://" + b + ".cos." + strings.TrimSpace(region) + ".myqcloud.com"
}

// PresignGet 生成对象的读预签名 URL（私有桶，带签名，可公网直接 GET）。
func (s *Store) PresignGet(ctx context.Context, key string, expire time.Duration) (string, error) {
	u, err := s.client.Object.GetPresignedURL(ctx, http.MethodGet, key, s.secretID, s.secretKey, expire, nil)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

// Upload 上传对象（r 读到 EOF）。
func (s *Store) Upload(ctx context.Context, key string, r io.Reader) error {
	_, err := s.client.Object.Put(ctx, key, r, nil)
	return err
}

// Head 返回对象的元数据（HEAD 权限与 GET 一致）；用于诊断读权限与对象存在性。
func (s *Store) Head(ctx context.Context, key string) error {
	resp, err := s.client.Object.Head(ctx, key, nil)
	if err != nil {
		return err
	}
	return resp.Body.Close()
}

// Probe 探测对象是否可读（Range GET bytes=0-0）。
// 返回 true 表示可访问（206）；false 表示不可访问（COS 欠费/停服 451、对象不存在或其他错误）。
// 供调用方在 COS 不可用时自动回退文件服务。
func (s *Store) Probe(ctx context.Context, key string) bool {
	u, err := s.PresignGet(ctx, key, time.Minute)
	if err != nil {
		return false
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return false
	}
	req.Header.Set("Range", "bytes=0-0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusPartialContent // 206
}
