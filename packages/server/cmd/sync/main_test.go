package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
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
