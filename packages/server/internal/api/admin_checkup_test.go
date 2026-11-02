package api

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestAdminCourseCheckup(t *testing.T) {
	server, database, token, _ := adminDetailServer(t)
	defer database.Close()
	dir := t.TempDir()
	server.cfg.ContentDir = dir
	if err := os.MkdirAll(filepath.Join(dir, "kb"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "kb", "l1.md"), []byte("# 讲义"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "kb", "l2.md"), []byte("# 讲义二"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO asset(lecture_id,kind,title,storage_key) VALUES(1,'markdown','讲义一','kb/l1.md'),(2,'markdown','讲义二','kb/l2.md')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO question(course_id,qno,level,type,gradability,stem,answer,score) VALUES(1,'L1-01-01','L1','single','auto','题干','A',1)`); err != nil {
		t.Fatal(err)
	}
	r := server.Router()
	got := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/courses/C1/checkup", "", token)
	if got.Code != http.StatusOK || !containsAll(got.Body.String(), `"ready":true`, `"label":"讲次完整"`, `"question_levels"`) {
		t.Fatalf("内容体检异常: %d %s", got.Code, got.Body.String())
	}
	if err := os.Remove(filepath.Join(dir, "kb", "l2.md")); err != nil {
		t.Fatal(err)
	}
	missing := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/courses/C1/checkup", "", token)
	if missing.Code != http.StatusOK || !containsAll(missing.Body.String(), `"ready":false`, `"缺失文件 1"`) {
		t.Fatalf("缺失文件应体检失败: %d %s", missing.Code, missing.Body.String())
	}
	if got := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/courses/NOPE/checkup", "", token); got.Code != http.StatusNotFound {
		t.Fatalf("不存在课程应 404: %d", got.Code)
	}
}
