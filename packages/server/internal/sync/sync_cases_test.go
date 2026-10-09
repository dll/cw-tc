package sync

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSyncCaseBasedCourseSetsAuxLineAndSyncesCases(t *testing.T) {
	database, cleanup := syncDB(t)
	defer cleanup()
	root := t.TempDir()
	courseDir := filepath.Join(root, "AI化教学创新02-DO")
	if err := os.MkdirAll(filepath.Join(courseDir, "01-案例A-g0atm自建DevOps栈"), 0o755); err != nil {
		t.Fatal(err)
	}
	overview := Overview{
		Courses: []Course{
			{
				ID: "02-DO", Dir: "AI化教学创新02-DO", Path: courseDir, Exists: true, Type: "案例制", Lectures: 0,
				Testbank: &Testbank{Title: "《软件工程基础 · DevOps 新范式》分级测试题库", CertNames: []string{"a", "b", "c"}, Levels: map[string]Level{"L1": {Pass: 60}}},
			},
		},
	}
	body, _ := json.Marshal(overview)
	if err := os.WriteFile(filepath.Join(root, overviewFile), body, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Run(context.Background(), database, root); err != nil {
		t.Fatal(err)
	}
	var line string
	if err := database.QueryRow(`SELECT line FROM course WHERE course_code='02-DO' AND source='upstream'`).Scan(&line); err != nil {
		t.Fatalf("案例制课程未写入: %v", err)
	}
	if line != "aux" {
		t.Fatalf("案例制课程 line 应为 aux，实际 %s", line)
	}
	var count int
	if err := database.QueryRow(`SELECT COUNT(*) FROM lecture`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("案例制课程应同步案例讲次，实际 %d", count)
	}
	var title string
	if err := database.QueryRow(`SELECT title FROM lecture WHERE no = 1`).Scan(&title); err != nil {
		t.Fatal(err)
	}
	if title != "案例A-g0atm自建DevOps栈" {
		t.Fatalf("案例讲次标题应去掉前导连字符，实际 %q", title)
	}
}

func TestSyncPrunesRemovedUpstreamCourse(t *testing.T) {
	database, cleanup := syncDB(t)
	defer cleanup()
	root := buildFixture(t)
	if err := Run(context.Background(), database, root); err != nil {
		t.Fatal(err)
	}
	// 上游只剩空课程列表 → 课程被清理
	if err := os.WriteFile(filepath.Join(root, overviewFile), []byte(`{"courses":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Run(context.Background(), database, root); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := database.QueryRow(`SELECT COUNT(*) FROM course WHERE source='upstream'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("上游删课后应清理上游课程，实际 %d", count)
	}
}
