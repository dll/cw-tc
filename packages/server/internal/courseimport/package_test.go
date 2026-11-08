package courseimport

import (
	"archive/zip"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeZip(t *testing.T, path string, files map[string]string) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	for name, content := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func samplePackageFiles() map[string]string {
	return map[string]string{
		"manifest.json": `{
  "course_code": "PY-201",
  "name": "Python 进阶",
  "line": "main",
  "price_cents": 12900,
  "teacher_id": 1,
  "lectures": [
    {"no": 1, "title": "第一讲", "is_preview": true, "assets": [
      {"title": "讲义", "path": "01/notes.md"},
      {"title": "课件", "path": "01/slides"}
    ]}
  ]
}`,
		"01/notes.md":              "# 第一讲\n讲义",
		"01/slides/index.html":     `<html><link href="assets/app.css"></html>`,
		"01/slides/assets/app.css": "body{color:red}",
	}
}

func TestExtractPackageAndImport(t *testing.T) {
	database, contentDir := setup(t)
	defer database.Close()
	root := t.TempDir()
	zipPath := filepath.Join(root, "course.zip")
	writeZip(t, zipPath, samplePackageFiles())
	extractDir := filepath.Join(root, "extract")
	pkgRoot, err := ExtractPackage(zipPath, extractDir)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := ReadManifest(pkgRoot)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.CourseCode != "PY-201" || len(manifest.Lectures) != 1 {
		t.Fatalf("清单解析异常: %+v", manifest)
	}
	manifest.SourceRoot = "."
	importer := &Importer{DB: database, ContentDir: contentDir, ImportRoot: pkgRoot}
	result, err := importer.Import(context.Background(), manifest)
	if err != nil {
		t.Fatal(err)
	}
	if result.Lectures != 1 || result.Assets != 2 {
		t.Fatalf("导入计数异常: %+v", result)
	}
	if !fileExists(filepath.Join(contentDir, "courses", "PY-201", "lectures", "1", "html", "assets", "app.css")) {
		t.Fatal("HTML 目录的 assets 未随包导入")
	}
}

func TestExtractPackageSupportsSingleTopDirectory(t *testing.T) {
	root := t.TempDir()
	zipPath := filepath.Join(root, "course.zip")
	files := map[string]string{}
	for name, content := range samplePackageFiles() {
		files["PY-201/"+name] = content
	}
	writeZip(t, zipPath, files)
	pkgRoot, err := ExtractPackage(zipPath, filepath.Join(root, "extract"))
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(pkgRoot) != "PY-201" {
		t.Fatalf("应识别唯一顶层目录，实际 %s", pkgRoot)
	}
	if _, err := ReadManifest(pkgRoot); err != nil {
		t.Fatal(err)
	}
}

func TestExtractPackageRejectsZipSlipAndMissingManifest(t *testing.T) {
	root := t.TempDir()
	// 路径穿越
	zipPath := filepath.Join(root, "slip.zip")
	writeZip(t, zipPath, map[string]string{"../evil.txt": "x", "manifest.json": "{}"})
	if _, err := ExtractPackage(zipPath, filepath.Join(root, "slip")); !errors.Is(err, ErrPackage) {
		t.Fatalf("路径穿越应拒绝: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "evil.txt")); !os.IsNotExist(err) {
		t.Fatal("路径穿越文件不应被写出")
	}
	// 缺清单
	zipPath = filepath.Join(root, "nomanifest.zip")
	writeZip(t, zipPath, map[string]string{"01/notes.md": "# x"})
	if _, err := ExtractPackage(zipPath, filepath.Join(root, "nomanifest")); !errors.Is(err, ErrPackage) || !strings.Contains(err.Error(), "manifest.json") {
		t.Fatalf("缺清单应拒绝并说明原因: %v", err)
	}
}

func TestReadManifestRejectsInvalidJSON(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "manifest.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadManifest(root); !errors.Is(err, ErrPackage) {
		t.Fatalf("非法 JSON 应拒绝: %v", err)
	}
}
