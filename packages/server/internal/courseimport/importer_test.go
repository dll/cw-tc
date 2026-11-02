package courseimport

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cw-tc/server/internal/platform/db"
	"github.com/cw-tc/server/migrations"
)

func setup(t *testing.T) (*sql.DB, string) {
	t.Helper()
	database, err := db.Open(context.Background(), db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.ApplyMigrations(context.Background(), database, migrations.FS, db.DialectSQLite); err != nil {
		database.Close()
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO staff(id,account,name,role,password_hash,created_source) VALUES(1,'t1','教师','teacher',X'01','seed')`); err != nil {
		database.Close()
		t.Fatal(err)
	}
	return database, t.TempDir()
}

func writeFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
}

func sampleImport(t *testing.T, contentDir string) string {
	t.Helper()
	root := filepath.Join(contentDir, "imports")
	courseDir := filepath.Join(root, "PY-101")
	writeFile(t, filepath.Join(courseDir, "01", "video.mp4"), []byte("fake-mp4-data"))
	writeFile(t, filepath.Join(courseDir, "01", "notes.md"), []byte("# 第一讲\n讲义内容"))
	writeFile(t, filepath.Join(courseDir, "01", "slides", "index.html"), []byte("<html><link href=\"assets/app.css\"></html>"))
	writeFile(t, filepath.Join(courseDir, "01", "slides", "assets", "app.css"), []byte("body{color:red}"))
	writeFile(t, filepath.Join(courseDir, "02", "audio.mp3"), []byte("fake-mp3-data"))
	return root
}

func sampleManifest() Manifest {
	return Manifest{
		CourseCode: "PY-101", Name: "Python 入门", Intro: "样例课程", Line: "main",
		PriceCents: 9900, TeacherID: int64Ptr(1),
		Lectures: []Lecture{
			{No: 1, Title: "第一讲", IsPreview: true, Assets: []Asset{
				{Kind: "video", Title: "视频", Path: "01/video.mp4"},
				{Kind: "", Title: "讲义", Path: "01/notes.md"},
				{Kind: "html", Title: "课件", Path: "01/slides"},
			}},
			{No: 2, Title: "第二讲", Assets: []Asset{
				{Kind: "audio", Title: "音频", Path: "02/audio.mp3"},
			}},
		},
	}
}

func TestImportMixedResources(t *testing.T) {
	database, contentDir := setup(t)
	defer database.Close()
	root := sampleImport(t, contentDir)
	importer := &Importer{DB: database, ContentDir: contentDir, ImportRoot: root}
	result, err := importer.Import(context.Background(), sampleManifest())
	if err != nil {
		t.Fatal(err)
	}
	if result.Lectures != 2 || result.Assets != 4 {
		t.Fatalf("导入计数异常: %+v", result)
	}
	var source, status, name string
	var price int64
	var lectureCount int
	if err := database.QueryRow(`SELECT source, status, name, price_cents, lecture_count FROM course WHERE course_code='PY-101'`).
		Scan(&source, &status, &name, &price, &lectureCount); err != nil {
		t.Fatal(err)
	}
	if source != "platform" || status != "building" || name != "Python 入门" || price != 9900 || lectureCount != 2 {
		t.Fatalf("课程落库异常: %s %s %s %d %d", source, status, name, price, lectureCount)
	}
	var kinds, keys []string
	rows, err := database.Query(`SELECT a.kind, a.storage_key, a.size_bytes, a.sha256 FROM asset a ORDER BY a.lecture_id, a.sort`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var kind, key, sha string
		var size int64
		if err := rows.Scan(&kind, &key, &size, &sha); err != nil {
			t.Fatal(err)
		}
		kinds = append(kinds, kind)
		keys = append(keys, key)
		if size <= 0 || sha == "" {
			t.Fatalf("资源大小 / 哈希未写入: %s %d %s", key, size, sha)
		}
	}
	if len(kinds) != 4 || !contains(kinds, "video") || !contains(kinds, "markdown") || !contains(kinds, "html") || !contains(kinds, "audio") {
		t.Fatalf("资源类型异常: %v", kinds)
	}
	for _, key := range keys {
		if !strings.HasPrefix(key, "courses/PY-101/lectures/") {
			t.Fatalf("存储键不符合课程目录规范: %s", key)
		}
		if !fileExists(filepath.Join(contentDir, filepath.FromSlash(key))) {
			t.Fatalf("资源文件未落盘: %s", key)
		}
	}
	// HTML 目录的相对依赖必须一起复制。
	if !fileExists(filepath.Join(contentDir, "courses", "PY-101", "lectures", "1", "html", "assets", "app.css")) {
		t.Fatal("HTML 目录 assets 未复制")
	}
}

func TestImportRejectsDuplicatesMissingAndUnsupported(t *testing.T) {
	database, contentDir := setup(t)
	defer database.Close()
	root := sampleImport(t, contentDir)
	importer := &Importer{DB: database, ContentDir: contentDir, ImportRoot: root}
	ctx := context.Background()
	if _, err := importer.Import(ctx, sampleManifest()); err != nil {
		t.Fatal(err)
	}
	if _, err := importer.Import(ctx, sampleManifest()); !errors.Is(err, ErrCourseExists) {
		t.Fatalf("重复课程应拒绝: %v", err)
	}

	m := sampleManifest()
	m.CourseCode = "PY-102"
	m.Lectures[0].Assets[0].Path = "01/missing.mp4"
	if _, err := importer.Import(ctx, m); !errors.Is(err, ErrSourceNotFound) {
		t.Fatalf("缺失文件应拒绝: %v", err)
	}

	m = sampleManifest()
	m.CourseCode = "PY-103"
	m.Lectures[0].Assets[0].Path = "../../etc/passwd"
	if _, err := importer.Import(ctx, m); !errors.Is(err, ErrSourceNotFound) {
		t.Fatalf("越界路径应拒绝: %v", err)
	}

	m = sampleManifest()
	m.CourseCode = "PY-104"
	m.SourceRoot = "PY-101"
	m.Lectures[0].Assets = []Asset{{Title: "未知", Path: "01/notes.md", Kind: "bad"}}
	if _, err := importer.Import(ctx, m); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("未知资源类型应拒绝: %v", err)
	}

	writeFile(t, filepath.Join(root, "PY-105", "x", "slide.html"), []byte("x"))
	m = sampleManifest()
	m.CourseCode = "PY-105"
	m.Lectures = []Lecture{{No: 1, Title: "第一讲", Assets: []Asset{{Path: "x", Kind: "html"}}}}
	if _, err := importer.Import(ctx, m); err == nil {
		t.Fatal("HTML 目录缺少 index.html 应拒绝")
	}
}

type fakeUploader struct{ keys []string }

func (f *fakeUploader) Upload(_ context.Context, key string, r io.Reader) error {
	f.keys = append(f.keys, key)
	_, _ = io.Copy(io.Discard, r)
	return nil
}

func TestImportUploadsMediaToCOS(t *testing.T) {
	database, contentDir := setup(t)
	defer database.Close()
	root := sampleImport(t, contentDir)
	uploader := &fakeUploader{}
	importer := &Importer{DB: database, ContentDir: contentDir, ImportRoot: root, Uploader: uploader}
	if _, err := importer.Import(context.Background(), sampleManifest()); err != nil {
		t.Fatal(err)
	}
	if len(uploader.keys) != 2 {
		t.Fatalf("MP4 / 音频应上传 COS: %v", uploader.keys)
	}
	for _, key := range uploader.keys {
		if strings.Contains(key, "/html/") || strings.Contains(key, "/markdown/") {
			t.Fatalf("非媒体资源不应上传 COS: %s", key)
		}
		if fileExists(filepath.Join(contentDir, filepath.FromSlash(key))) {
			t.Fatalf("已上传 COS 的媒体不应再落本地: %s", key)
		}
	}
}

func int64Ptr(v int64) *int64 { return &v }

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func TestImportRequiresTeacherOwner(t *testing.T) {
	database, contentDir := setup(t)
	defer database.Close()
	if _, err := database.Exec(`INSERT INTO staff(id,account,name,role,password_hash,created_source) VALUES(2,'ops','运营','admin_ops',X'01','seed')`); err != nil {
		t.Fatal(err)
	}
	root := sampleImport(t, contentDir)
	importer := &Importer{DB: database, ContentDir: contentDir, ImportRoot: root}
	m := sampleManifest()
	m.CourseCode = "PY-200"
	m.TeacherID = int64Ptr(2)
	if _, err := importer.Import(context.Background(), m); !errors.Is(err, ErrInput) {
		t.Fatalf("非教师账号不得作为课程归属: %v", err)
	}
}

func TestDetectKindAndStorageKey(t *testing.T) {
	cases := map[string]string{".md": "markdown", ".mp4": "video", ".mp3": "audio", ".pdf": "slide", ".py": "code", ".json": "frozen_output", ".html": "html"}
	for ext, want := range cases {
		got, err := DetectKind("x"+ext, "")
		if err != nil || got != want {
			t.Fatalf("%s 应识别为 %s，实际 %s %v", ext, want, got, err)
		}
	}
	if got, err := DetectKind("x.bin", "slide"); err != nil || got != "slide" {
		t.Fatalf("显式类型应优先: %s %v", got, err)
	}
	if _, err := DetectKind("x.bin", "bad"); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("非法显式类型应拒绝: %v", err)
	}
	if _, err := DetectKind("x.bin", ""); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("未知扩展名应拒绝: %v", err)
	}
	if got := StorageKey("PY-1", 2, "video", "a.mp4"); got != "courses/PY-1/lectures/2/video/a.mp4" {
		t.Fatalf("存储键异常: %s", got)
	}
}
