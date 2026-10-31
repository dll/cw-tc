package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSyncCoverageBranches(t *testing.T) {
	ctx := context.Background()
	if _, _, _, err := runSync(ctx, "mysql", "", t.TempDir()); err == nil {
		t.Fatal("未知数据库方言应报错")
	}
	dsn := filepath.Join(t.TempDir(), "sync.db")
	source := filepath.Join(t.TempDir(), "empty-source")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := runSync(ctx, "sqlite", dsn, source); err == nil {
		t.Fatal("空内容目录应同步失败（不静默成功）")
	}
}

func TestMediaValidationBranches(t *testing.T) {
	ctx := context.Background()
	dsn := filepath.Join(t.TempDir(), "media.db")
	if _, err := importMedia(ctx, "mysql", dsn, "x.json"); err == nil {
		t.Fatal("未知方言应报错")
	}
	if _, err := importMedia(ctx, "sqlite", dsn, filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Fatal("清单文件不存在应报错")
	}
	bad := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(bad, []byte("not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := importMedia(ctx, "sqlite", dsn, bad); err == nil {
		t.Fatal("非法 JSON 应报错")
	}
	empty := filepath.Join(t.TempDir(), "empty.json")
	if err := os.WriteFile(empty, []byte("[]"), 0o600); err != nil {
		t.Fatal(err)
	}
	n, err := importMedia(ctx, "sqlite", dsn, empty)
	if err != nil || n != 0 {
		t.Fatalf("空清单应导入 0 条: %d %v", n, err)
	}
}
