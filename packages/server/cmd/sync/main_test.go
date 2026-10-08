package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/cw-tc/server/internal/sync"
)

func TestRunSyncWithFixture(t *testing.T) {
	root := t.TempDir()
	courseDir := filepath.Join(root, "AI化教学创新03-CF")
	if err := os.MkdirAll(filepath.Join(courseDir, "01_你好世界"), 0o755); err != nil {
		t.Fatal(err)
	}
	overview := map[string]any{
		"courses": []map[string]any{
			{
				"id": "03-CF", "dir": "AI化教学创新03-CF", "path": courseDir, "exists": true, "type": "讲次制", "lectures": 1,
				"testbank": map[string]any{"title": "《C语言程序设计》分级测试题库", "cert_names": []string{"a", "b", "c"}, "levels": map[string]any{"L1": map[string]any{"pass": 60}}},
			},
		},
	}
	body, _ := json.Marshal(overview)
	if err := os.WriteFile(filepath.Join(root, "课程总览.json"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	courses, lectures, questions, err := runSync(context.Background(), "", "", root)
	if err != nil {
		t.Fatal(err)
	}
	if courses != 1 || lectures != 1 {
		t.Fatalf("同步结果错误: 课程 %d 讲次 %d 题目 %d", courses, lectures, questions)
	}
}

func TestImportMediaErrors(t *testing.T) {
	ctx := context.Background()
	// 缺 manifest 文件
	if _, err := importMedia(ctx, "", "", filepath.Join(t.TempDir(), "nope.json")); err == nil {
		t.Fatal("缺 manifest 应报错")
	}
	// 非法 JSON
	bad := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(bad, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := importMedia(ctx, "", "", bad); err == nil {
		t.Fatal("非法 JSON 应报错")
	}
	// 非法驱动
	if _, err := importMedia(ctx, "oracle", "", bad); err == nil {
		t.Fatal("非法驱动应报错")
	}
}

func TestImportMediaHappyPath(t *testing.T) {
	// 用文件 sqlite 让 runSync 与 importMedia 共享同一个库
	dbFile := filepath.Join(t.TempDir(), "test.db")
	dsn := "file:" + dbFile

	root := t.TempDir()
	courseDir := filepath.Join(root, "AI化教学创新03-CF")
	if err := os.MkdirAll(filepath.Join(courseDir, "01_你好世界"), 0o755); err != nil {
		t.Fatal(err)
	}
	overview := map[string]any{
		"courses": []map[string]any{
			{"id": "03-CF", "dir": "AI化教学创新03-CF", "path": courseDir, "exists": true, "type": "讲次制", "lectures": 1,
				"testbank": map[string]any{"title": "t", "cert_names": []string{"a"}, "levels": map[string]any{"L1": map[string]any{"pass": 60}}}},
		},
	}
	body, _ := json.Marshal(overview)
	if err := os.WriteFile(filepath.Join(root, "课程总览.json"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := runSync(context.Background(), "sqlite", dsn, root); err != nil {
		t.Fatal(err)
	}

	manifest := filepath.Join(t.TempDir(), "m.json")
	entries := []sync.MediaEntry{
		{StorageKey: "AI化教学创新03-CF/01_你好世界/video/讲解.mp4", Kind: "video", Size: 123, CourseCode: "03-CF", LectureNo: 1},
	}
	mb, _ := json.Marshal(entries)
	if err := os.WriteFile(manifest, mb, 0o644); err != nil {
		t.Fatal(err)
	}
	n, err := importMedia(context.Background(), "sqlite", dsn, manifest)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("应导入 1 条，实际 %d", n)
	}
}
