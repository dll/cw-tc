package api

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestAdminKnowledgeRebuildAndStatus(t *testing.T) {
	server, database, token, _ := adminDetailServer(t)
	defer database.Close()
	dir := t.TempDir()
	server.cfg.ContentDir = dir
	if err := os.MkdirAll(filepath.Join(dir, "kb"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "kb", "l1.md"), []byte("# 分布式基础\n\n分布式系统需要一致性与可用性。"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO asset(lecture_id,kind,title,storage_key) VALUES(1,'markdown','讲义','kb/l1.md')`); err != nil {
		t.Fatal(err)
	}
	r := server.Router()
	rebuild := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/courses/C1/kb/rebuild", `{}`, token)
	if rebuild.Code != http.StatusOK || !containsAll(rebuild.Body.String(), `"assets":1`, `"chunks":2`) {
		t.Fatalf("知识库重建失败: %d %s", rebuild.Code, rebuild.Body.String())
	}
	status := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/kb/status?course_code=C1", "", token)
	if status.Code != http.StatusOK || !containsAll(status.Body.String(), `"kb_status":"ready"`, `"chunks":2`, `"assets":1`) {
		t.Fatalf("知识库状态异常: %d %s", status.Code, status.Body.String())
	}
	var courseID int64
	_ = database.QueryRow(`SELECT id FROM course WHERE course_code='C1'`).Scan(&courseID)
	// 删除源文件后仍应能从索引检索（证明知识库切片生效）。
	_ = os.Remove(filepath.Join(dir, "kb", "l1.md"))
	contextText, citations := server.courseContext(context.Background(), courseID, 1, "分布式系统的核心是什么")
	if contextText == "" || len(citations) != 1 || citations[0].LectureNo != 1 {
		t.Fatalf("知识库检索未生效: %q %+v", contextText, citations)
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/courses/NOPE/kb/rebuild", `{}`, token); got.Code != http.StatusNotFound {
		t.Fatalf("不存在课程应 404，实际 %d", got.Code)
	}
	server.cfg.ContentDir = ""
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/courses/C1/kb/rebuild", `{}`, token); got.Code != http.StatusServiceUnavailable {
		t.Fatalf("未配置内容目录应 503，实际 %d", got.Code)
	}
	var audits int
	if err := database.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action='course.kb_rebuild'`).Scan(&audits); err != nil || audits != 1 {
		t.Fatalf("重建必须写审计: %d %v", audits, err)
	}
}
