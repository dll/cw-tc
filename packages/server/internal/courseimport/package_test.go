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

type zipEntry struct {
	name         string
	body         string
	mode         os.FileMode
	method       uint16
	raw          bool
	crc          uint32
	compressed   uint64
	uncompressed uint64
}

func writeZipEntries(t *testing.T, path string, entries []zipEntry) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	for _, item := range entries {
		header := &zip.FileHeader{Name: item.name, Method: item.method}
		if item.mode != 0 {
			header.SetMode(item.mode)
		}
		if item.raw {
			header.CRC32 = item.crc
			header.CompressedSize64 = item.compressed
			header.UncompressedSize64 = item.uncompressed
			target, err := writer.CreateRaw(header)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := target.Write([]byte(item.body)); err != nil {
				t.Fatal(err)
			}
			continue
		}
		target, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := target.Write([]byte(item.body)); err != nil {
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

func TestExtractPackageRejectsBrokenArchives(t *testing.T) {
	root := t.TempDir()
	notZip := filepath.Join(root, "not.zip")
	if err := os.WriteFile(notZip, []byte("not a zip"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ExtractPackage(notZip, filepath.Join(root, "out1")); !errors.Is(err, ErrPackage) {
		t.Fatalf("非 ZIP 应拒绝: %v", err)
	}

	destFile := filepath.Join(root, "dest-file")
	if err := os.WriteFile(destFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	zipPath := filepath.Join(root, "ok.zip")
	writeZip(t, zipPath, map[string]string{"manifest.json": "{}"})
	if _, err := ExtractPackage(zipPath, destFile); err == nil {
		t.Fatal("目标目录是文件时应失败")
	}
}

func TestExtractPackageSkipsNoiseAndSymlinks(t *testing.T) {
	root := t.TempDir()
	zipPath := filepath.Join(root, "noise.zip")
	writeZipEntries(t, zipPath, []zipEntry{
		{name: "__MACOSX/._x", body: "x"},
		{name: "dir/.DS_Store", body: "x"},
		{name: "a/..", body: "x"},
		{name: "docs/", body: "", mode: os.ModeDir | 0o755},
		{name: "link", body: "docs", mode: os.ModeSymlink | 0o777},
		{name: "manifest.json", body: "{}"},
	})
	pkgRoot, err := ExtractPackage(zipPath, filepath.Join(root, "out"))
	if err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(filepath.Join(pkgRoot, "docs")); err != nil || !info.IsDir() {
		t.Fatalf("目录项应创建: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(pkgRoot, "link")); !os.IsNotExist(err) {
		t.Fatal("符号链接项应跳过")
	}
	if _, err := os.Stat(filepath.Join(pkgRoot, "__MACOSX")); !os.IsNotExist(err) {
		t.Fatal("__MACOSX 应跳过")
	}
	if _, err := os.Stat(filepath.Join(pkgRoot, "dir")); !os.IsNotExist(err) {
		t.Fatal(".DS_Store 所在目录不应因该文件创建")
	}
}

func TestExtractPackageRejectsOversizedEntry(t *testing.T) {
	root := t.TempDir()
	zipPath := filepath.Join(root, "big.zip")
	writeZipEntries(t, zipPath, []zipEntry{
		{name: "big.bin", body: "x", raw: true, method: zip.Store, compressed: 1, uncompressed: uint64(MaxPackageFileBytes) + 1},
		{name: "manifest.json", body: "{}"},
	})
	if _, err := ExtractPackage(zipPath, filepath.Join(root, "out")); !errors.Is(err, ErrPackage) {
		t.Fatalf("超大单文件应拒绝: %v", err)
	}
}

func TestExtractPackageReportsEntryFailures(t *testing.T) {
	root := t.TempDir()

	t.Run("未知压缩方法", func(t *testing.T) {
		zipPath := filepath.Join(root, "method.zip")
		writeZipEntries(t, zipPath, []zipEntry{
			{name: "weird.bin", body: "x", raw: true, method: 99, compressed: 1, uncompressed: 1},
			{name: "manifest.json", body: "{}"},
		})
		if _, err := ExtractPackage(zipPath, filepath.Join(root, "method")); err == nil {
			t.Fatal("未知压缩方法应失败")
		}
	})

	t.Run("声明长度不一致", func(t *testing.T) {
		zipPath := filepath.Join(root, "crc.zip")
		writeZipEntries(t, zipPath, []zipEntry{
			{name: "bad.bin", body: "hello", raw: true, method: zip.Store, compressed: 5, uncompressed: 3},
			{name: "manifest.json", body: "{}"},
		})
		if _, err := ExtractPackage(zipPath, filepath.Join(root, "crc")); err == nil {
			t.Fatal("声明长度不一致应失败")
		}
	})

	t.Run("目标已是目录", func(t *testing.T) {
		zipPath := filepath.Join(root, "dup.zip")
		writeZipEntries(t, zipPath, []zipEntry{
			{name: "dup/", body: "", mode: os.ModeDir | 0o755},
			{name: "dup", body: "x"},
			{name: "manifest.json", body: "{}"},
		})
		if _, err := ExtractPackage(zipPath, filepath.Join(root, "dup")); err == nil {
			t.Fatal("同名目录与文件应失败")
		}
	})
}

func TestExtractPackageReportsCreateFailures(t *testing.T) {
	root := t.TempDir()

	t.Run("父路径是文件", func(t *testing.T) {
		out := filepath.Join(root, "out-parent")
		if err := os.MkdirAll(out, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(out, "parent"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		zipPath := filepath.Join(root, "parent.zip")
		writeZipEntries(t, zipPath, []zipEntry{
			{name: "parent/child.txt", body: "x"},
			{name: "manifest.json", body: "{}"},
		})
		if _, err := ExtractPackage(zipPath, out); err == nil {
			t.Fatal("父路径是文件时应失败")
		}
	})

	t.Run("目录项位置是文件", func(t *testing.T) {
		out := filepath.Join(root, "out-dir")
		if err := os.MkdirAll(out, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(out, "d"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		zipPath := filepath.Join(root, "dir.zip")
		writeZipEntries(t, zipPath, []zipEntry{
			{name: "d/", body: "", mode: os.ModeDir | 0o755},
			{name: "manifest.json", body: "{}"},
		})
		if _, err := ExtractPackage(zipPath, out); err == nil {
			t.Fatal("目录项位置是文件时应失败")
		}
	})
}

func TestReadManifestAndFindRootErrors(t *testing.T) {
	root := t.TempDir()
	if _, err := ReadManifest(root); !errors.Is(err, ErrPackage) {
		t.Fatalf("缺少 manifest.json 应拒绝: %v", err)
	}
	if _, err := findManifestRoot(filepath.Join(root, "missing")); err == nil {
		t.Fatal("读取不存在的解压目录应失败")
	}
}

func TestCleanupLocalRemovesFilesAndDirectories(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "file.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "dir")
	if err := os.MkdirAll(filepath.Join(dir, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	cleanupLocal([]string{file, dir})
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatal("文件应被清理")
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("目录应被清理")
	}
}
