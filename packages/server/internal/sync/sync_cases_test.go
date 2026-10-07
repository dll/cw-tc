package sync

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSyncCaseBasedCourseSetsAuxLineAndNoLectures(t *testing.T) {
	database, cleanup := syncDB(t)
	defer cleanup()
	root := t.TempDir()
	overview := Overview{
		Courses: []Course{
			{
				ID: "02-DO", Dir: "AI化教学创新02-DO", Path: filepath.Join(root, "AI化教学创新02-DO"), Exists: true, Type: "案例制", Lectures: 0,
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
	var count int
	if err := database.QueryRow(`SELECT line FROM course WHERE course_code='02-DO' AND source='upstream'`).Scan(&line); err != nil {
		t.Fatalf("案例制课程未写入: %v", err)
	}
	if line != "aux" {
		t.Fatalf("案例制课程 line 应为 aux，实际 %s", line)
	}
	if err := database.QueryRow(`SELECT COUNT(*) FROM lecture`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("案例制课程不应有讲次，实际 %d", count)
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
