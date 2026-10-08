package main

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/cw-tc/server/internal/sync"
)

func TestScanMedia(t *testing.T) {
	root := t.TempDir()
	overview := `{"courses":[{"id":"03-CF","dir":"AI化教学创新03-CF"}]}`
	if err := os.WriteFile(filepath.Join(root, "课程总览.json"), []byte(overview), 0o644); err != nil {
		t.Fatal(err)
	}
	videoDir := filepath.Join(root, "AI化教学创新03-CF", "01_你好世界", "video")
	if err := os.MkdirAll(videoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(videoDir, "讲解.mp4"), []byte("v"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(videoDir, "讲解.mp3"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	// 非媒体文件应被忽略
	if err := os.WriteFile(filepath.Join(root, "AI化教学创新03-CF", "01_你好世界", "README.md"), []byte("m"), 0o644); err != nil {
		t.Fatal(err)
	}

	entries, err := scanMedia(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("应扫到 2 个媒体文件，实际 %d", len(entries))
	}
	kinds := map[string]bool{}
	for _, e := range entries {
		kinds[e.Kind] = true
		if e.CourseCode != "03-CF" || e.LectureNo != 1 {
			t.Fatalf("映射错误: %+v", e)
		}
		if e.StorageKey == "" {
			t.Fatalf("storage_key 为空")
		}
	}
	if !kinds["video"] || !kinds["audio"] {
		t.Fatalf("应含 video + audio，实际 %v", kinds)
	}
}

type fakeUploader struct {
	uploaded map[string]bool
	failOn   string
}

func (f *fakeUploader) Upload(_ context.Context, key string, _ io.Reader) error {
	if key == f.failOn {
		return errors.New("boom")
	}
	f.uploaded[key] = true
	return nil
}

func TestRunUpload(t *testing.T) {
	root := t.TempDir()
	for name, content := range map[string]string{"a.mp4": "v", "b.mp3": "a", "c.wav": "w"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	entries := []sync.MediaEntry{
		{StorageKey: "a.mp4", Kind: "video"},
		{StorageKey: "b.mp3", Kind: "audio"},
		{StorageKey: "missing.mp4", Kind: "video"}, // 文件不存在 → 跳过
		{StorageKey: "c.wav", Kind: "audio"},       // 上传失败 → 跳过
	}
	up := &fakeUploader{uploaded: map[string]bool{}, failOn: "c.wav"}
	n := runUpload(context.Background(), root, entries, up)
	if n != 2 {
		t.Fatalf("应成功 2 个，实际 %d", n)
	}
	if !up.uploaded["a.mp4"] || !up.uploaded["b.mp3"] {
		t.Fatalf("a.mp4/b.mp3 应上传成功")
	}
	if up.uploaded["c.wav"] || up.uploaded["missing.mp4"] {
		t.Fatalf("c.wav/missing.mp4 不应上传成功")
	}
}

func TestRun(t *testing.T) {
	// 坏 source（缺课程总览.json）→ 扫描失败
	if _, err := run(filepath.Join(t.TempDir(), "nope"), "", &fakeUploader{uploaded: map[string]bool{}}); err == nil {
		t.Fatal("坏 source 应报错")
	}
	// 有效 source + nil uploader → 未配置报错
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "课程总览.json"), []byte(`{"courses":[{"id":"03-CF","dir":"AI化教学创新03-CF"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := run(root, "", nil); err == nil {
		t.Fatal("nil uploader 应报错")
	}
	// 有效 source + 假 uploader → 上传 + 写清单
	videoDir := filepath.Join(root, "AI化教学创新03-CF", "01_你好世界", "video")
	if err := os.MkdirAll(videoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(videoDir, "讲解.mp4"), []byte("v"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(t.TempDir(), "m.json")
	up := &fakeUploader{uploaded: map[string]bool{}}
	n, err := run(root, manifest, up)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("应上传 1 个，实际 %d", n)
	}
	if _, err := os.Stat(manifest); err != nil {
		t.Fatal("清单未写出")
	}
}
