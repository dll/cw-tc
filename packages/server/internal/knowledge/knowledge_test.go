package knowledge

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
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
	dir := t.TempDir()
	stmts := []string{
		`INSERT INTO staff(id,account,name,role,password_hash,created_source) VALUES(1,'t','教师','teacher',X'01','seed')`,
		`INSERT INTO major(id,name) VALUES(1,'专业')`,
		`INSERT INTO course(id,course_code,source,name,line,major_id,status,teacher_id) VALUES(1,'C1','platform','知识库课程','main',1,'published',1)`,
		`INSERT INTO lecture(id,course_id,no,title) VALUES(1,1,1,'第一讲'),(2,1,2,'第二讲')`,
		`INSERT INTO asset(id,lecture_id,kind,title,storage_key) VALUES(1,1,'markdown','讲义','kb/l1.md')`,
		`INSERT INTO asset(id,lecture_id,kind,title,storage_key) VALUES(2,1,'html','课件','kb/slides.html')`,
		`INSERT INTO asset(id,lecture_id,kind,title,storage_key) VALUES(3,2,'code','示例','kb/example.py')`,
	}
	for _, stmt := range stmts {
		if _, err := database.Exec(stmt); err != nil {
			database.Close()
			t.Fatal(err)
		}
	}
	files := map[string]string{
		"kb/l1.md":       "# 分布式基础\n\n分布式系统需要考虑一致性与可用性。",
		"kb/slides.html": "<html><body><h1>课程目标</h1><p>理解分布式系统。</p></body></html>",
		"kb/example.py":  "print('hello')\n# 示例代码",
	}
	for path, body := range files {
		full := filepath.Join(dir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return database, dir
}

func TestBuildSearchAndIdempotency(t *testing.T) {
	database, dir := setup(t)
	defer database.Close()
	ctx := context.Background()
	first, err := BuildCourse(ctx, database, dir, "C1")
	if err != nil {
		t.Fatal(err)
	}
	if first.Assets != 3 || first.Chunks < 3 || first.Chars == 0 {
		t.Fatalf("构建统计异常: %+v", first)
	}
	second, err := BuildCourse(ctx, database, dir, "C1")
	if err != nil || second.Chunks != first.Chunks {
		t.Fatalf("重建应幂等: %+v %v", second, err)
	}
	matches, err := Search(ctx, database, "C1", 1, []string{"分布式", "一致性"}, 8)
	if err != nil || len(matches) == 0 || matches[0].LectureNo != 1 || matches[0].StorageKey == "" {
		t.Fatalf("检索结果异常: %+v %v", matches, err)
	}
	if matches[0].LectureTitle != "第一讲" || matches[0].AssetTitle == "" {
		t.Fatalf("出处字段缺失: %+v", matches[0])
	}
	// HTML 标签已清理，不应把标签当正文。
	for _, match := range matches {
		if stringsHasTag(match.Content) {
			t.Fatalf("HTML 标签未清理: %s", match.Content)
		}
	}
	if none, err := Search(ctx, database, "C1", 2, []string{"完全不存在关键词"}, 8); err != nil || len(none) != 0 {
		t.Fatalf("无关关键词应无结果: %+v %v", none, err)
	}
	status, err := ListStatus(ctx, database, "C1")
	if err != nil || len(status) != 1 || status[0].KbStatus != "ready" || status[0].Chunks != first.Chunks || status[0].LastBuilt == nil {
		t.Fatalf("知识库状态异常: %+v %v", status, err)
	}
	if _, err := BuildCourse(ctx, database, dir, "NOPE"); !errors.Is(err, ErrCourseMissing) {
		t.Fatalf("不存在课程应拒绝: %v", err)
	}
	if _, err := BuildCourse(ctx, database, "", "C1"); !errors.Is(err, ErrInput) {
		t.Fatalf("空内容目录应拒绝: %v", err)
	}
	if _, err := Search(ctx, database, "", 1, nil, 8); !errors.Is(err, ErrInput) {
		t.Fatalf("空课程码检索应拒绝: %v", err)
	}
}

func stringsHasTag(value string) bool {
	for i := 0; i+1 < len(value); i++ {
		if value[i] == '<' && value[i+1] != ' ' {
			return true
		}
	}
	return false
}
