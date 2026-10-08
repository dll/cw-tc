package sync

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/cw-tc/server/internal/platform/db"
	"github.com/cw-tc/server/migrations"
)

// buildFixture 生成一个最小上游目录：一门讲次制课程（2 讲）。
func buildFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	courseDir := filepath.Join(root, "AI化教学创新03-CF")
	if err := os.MkdirAll(filepath.Join(courseDir, "01_你好世界"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(courseDir, "02_控制结构"), 0o755); err != nil {
		t.Fatal(err)
	}
	// 资源文件：讲义 + 课件（供 syncAssets 扫描）
	if err := os.WriteFile(filepath.Join(courseDir, "01_你好世界", "README.md"), []byte("# 讲义"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(courseDir, "01_你好世界", "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(courseDir, "01_你好世界", "docs", "课件.pptx"), []byte("PPT"), 0o644); err != nil {
		t.Fatal(err)
	}
	overview := Overview{
		Courses: []Course{
			{
				ID: "03-CF", Dir: "AI化教学创新03-CF", Path: courseDir, Exists: true, Type: "讲次制", Lectures: 2,
				Testbank: &Testbank{
					Title:     "《C语言程序设计》分级测试题库",
					CertNames: []string{"认证一", "认证二", "认证三"},
					Levels: map[string]Level{
						"L1": {Pass: 60}, "L2": {Pass: 70}, "L3": {Pass: 80},
					},
				},
			},
		},
	}
	body, err := json.Marshal(overview)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, overviewFile), body, 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func syncDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()
	database, err := db.Open(context.Background(), db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.ApplyMigrations(context.Background(), database, migrations.FS, db.DialectSQLite); err != nil {
		t.Fatal(err)
	}
	return database, func() { database.Close() }
}

func TestSyncWritesUpstreamCourseAndLectures(t *testing.T) {
	database, cleanup := syncDB(t)
	defer cleanup()
	root := buildFixture(t)
	if err := Run(context.Background(), database, root); err != nil {
		t.Fatal(err)
	}
	var name string
	var line string
	var lectureCount int
	if err := database.QueryRow(`SELECT name, line, lecture_count FROM course WHERE course_code='03-CF' AND source='upstream'`).Scan(&name, &line, &lectureCount); err != nil {
		t.Fatalf("上游课程未写入: %v", err)
	}
	if name != "C语言程序设计" || line != "main" || lectureCount != 2 {
		t.Fatalf("课程字段错误: %s %s %d", name, line, lectureCount)
	}
	var count int
	if err := database.QueryRow(`SELECT COUNT(*) FROM lecture l JOIN course c ON c.id=l.course_id WHERE c.course_code='03-CF' AND l.origin='upstream'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("应同步 2 讲，实际 %d", count)
	}
}

func TestSyncKeepsPlatformCourseAndAIAssistedLecture(t *testing.T) {
	database, cleanup := syncDB(t)
	defer cleanup()
	// 平台课程
	if _, err := database.Exec(`INSERT INTO staff(id,account,name,role,password_hash,created_source) VALUES(1,'t','教师','teacher','x','seed')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO course(course_code,source,name,line,status,teacher_id) VALUES('PLAT-1','platform','平台课','main','published',1)`); err != nil {
		t.Fatal(err)
	}
	// AI 增补讲次（挂在平台课下，origin='ai_assisted'）
	if _, err := database.Exec(`INSERT INTO lecture(course_id,no,title,origin) VALUES(1,99,'AI增补','ai_assisted')`); err != nil {
		t.Fatal(err)
	}

	root := buildFixture(t)
	if err := Run(context.Background(), database, root); err != nil {
		t.Fatal(err)
	}

	var platformCount int
	if err := database.QueryRow(`SELECT COUNT(*) FROM course WHERE source='platform'`).Scan(&platformCount); err != nil {
		t.Fatal(err)
	}
	if platformCount != 1 {
		t.Fatalf("platform 课程被误删: %d", platformCount)
	}
	var aiCount int
	if err := database.QueryRow(`SELECT COUNT(*) FROM lecture WHERE origin='ai_assisted'`).Scan(&aiCount); err != nil {
		t.Fatal(err)
	}
	if aiCount != 1 {
		t.Fatalf("ai_assisted 讲次被误删: %d", aiCount)
	}
}

func TestSyncPrunesRemovedUpstreamLecture(t *testing.T) {
	database, cleanup := syncDB(t)
	defer cleanup()
	root := buildFixture(t)
	if err := Run(context.Background(), database, root); err != nil {
		t.Fatal(err)
	}
	// 上游删掉第 2 讲：删目录后重跑
	if err := os.RemoveAll(filepath.Join(root, "AI化教学创新03-CF", "02_控制结构")); err != nil {
		t.Fatal(err)
	}
	if err := Run(context.Background(), database, root); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := database.QueryRow(`SELECT COUNT(*) FROM lecture l JOIN course c ON c.id=l.course_id WHERE c.course_code='03-CF' AND l.origin='upstream'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("上游删讲后应剩 1 讲，实际 %d", count)
	}
}

func TestSyncAssetsAndDefaultPrice(t *testing.T) {
	database, cleanup := syncDB(t)
	defer cleanup()
	root := buildFixture(t)
	if err := Run(context.Background(), database, root); err != nil {
		t.Fatal(err)
	}
	// 默认价格 5 畅币 = 5000 分
	var price int64
	if err := database.QueryRow(`SELECT price_cents FROM course WHERE course_code='03-CF' AND source='upstream'`).Scan(&price); err != nil {
		t.Fatal(err)
	}
	if price != defaultPriceCents {
		t.Fatalf("默认价应为 %d（5 畅币），实际 %d", defaultPriceCents, price)
	}
	// 资源落库（README.md + 课件.pptx）
	var n int
	if err := database.QueryRow(`SELECT COUNT(*) FROM asset a JOIN lecture l ON a.lecture_id=l.id WHERE l.no=1`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("应写入 2 条资源，实际 %d", n)
	}
	var kind string
	_ = database.QueryRow(`SELECT kind FROM asset WHERE storage_key LIKE '%README.md'`).Scan(&kind)
	if kind != "markdown" {
		t.Fatalf("README.md 应为 markdown，实际 %s", kind)
	}
}
