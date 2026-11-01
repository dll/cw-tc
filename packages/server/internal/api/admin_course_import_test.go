package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAdminImportCourseMixedResources(t *testing.T) {
	server, database, token, _ := adminDetailServer(t)
	defer database.Close()
	contentDir := t.TempDir()
	server.cfg.ContentDir = contentDir
	courseDir := filepath.Join(contentDir, "imports", "API-IMP-1")
	for _, item := range []struct{ path, body string }{
		{"01/notes.md", "# 第一讲\n讲义"},
		{"01/video.mp4", "fake-mp4"},
		{"01/slides/index.html", `<html><link href="assets/app.css"></html>`},
		{"01/slides/assets/app.css", "body{color:blue}"},
	} {
		full := filepath.Join(courseDir, filepath.FromSlash(item.path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(item.body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	manifest := `{
	  "course_code":"API-IMP-1","name":"导入样例课","intro":"来自资源包","line":"main",
	  "price_cents":12300,"teacher_id":2,"source_root":"API-IMP-1",
	  "lectures":[
	    {"no":1,"title":"第一讲","is_preview":true,"assets":[
	      {"title":"讲义","path":"01/notes.md"},
	      {"kind":"video","title":"视频","path":"01/video.mp4"},
	      {"kind":"html","title":"课件","path":"01/slides"}
	    ]}
	  ]
	}`
	r := server.Router()
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/courses/import",
		`{"confirm":false,"manifest":`+manifest+`}`, token); got.Code != http.StatusBadRequest {
		t.Fatalf("未确认导入应 400，实际 %d", got.Code)
	}
	ok := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/courses/import",
		`{"confirm":true,"manifest":`+manifest+`}`, token)
	if ok.Code != http.StatusOK || !containsAll(ok.Body.String(), `"course_code":"API-IMP-1"`, `"lectures":1`, `"assets":3`, `"status":"building"`) {
		t.Fatalf("课程导入失败: %d %s", ok.Code, ok.Body.String())
	}
	var source, status string
	var lectureCount int
	if err := database.QueryRow(`SELECT source, status, lecture_count FROM course WHERE course_code='API-IMP-1'`).
		Scan(&source, &status, &lectureCount); err != nil {
		t.Fatal(err)
	}
	if source != "platform" || status != "building" || lectureCount != 1 {
		t.Fatalf("课程落库异常: %s %s %d", source, status, lectureCount)
	}
	var kinds int
	if err := database.QueryRow(`SELECT COUNT(*) FROM asset a JOIN lecture l ON l.id=a.lecture_id JOIN course c ON c.id=l.course_id WHERE c.course_code='API-IMP-1'`).Scan(&kinds); err != nil || kinds != 3 {
		t.Fatalf("资源数量异常: %d %v", kinds, err)
	}
	if _, err := os.Stat(filepath.Join(contentDir, "courses/API-IMP-1/lectures/1/html/assets/app.css")); err != nil {
		t.Fatalf("HTML 依赖未复制: %v", err)
	}
	var audits int
	if err := database.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action='course.import'`).Scan(&audits); err != nil || audits != 1 {
		t.Fatalf("导入必须写审计: %d %v", audits, err)
	}
	if again := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/courses/import",
		`{"confirm":true,"manifest":`+manifest+`}`, token); again.Code != http.StatusConflict {
		t.Fatalf("重复导入应 409，实际 %d", again.Code)
	}
}

func TestImportedHTMLResourcesServedWithDependencies(t *testing.T) {
	server, database, token, _ := adminDetailServer(t)
	defer database.Close()
	contentDir := t.TempDir()
	server.cfg.ContentDir = contentDir
	courseDir := filepath.Join(contentDir, "imports", "HTML-1")
	for path, body := range map[string]string{
		"slides/index.html":     `<html><link href="assets/app.css"></html>`,
		"slides/assets/app.css": "body{color:green}",
	} {
		full := filepath.Join(courseDir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	manifest := `{"course_code":"HTML-1","name":"HTML 课件课","teacher_id":2,"source_root":"HTML-1",
	  "lectures":[{"no":1,"title":"第一讲","assets":[{"kind":"html","title":"课件","path":"slides"}]}]}`
	r := server.Router()
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/courses/import", `{"confirm":true,"manifest":`+manifest+`}`, token); got.Code != http.StatusOK {
		t.Fatalf("导入 HTML 课程失败: %d %s", got.Code, got.Body.String())
	}
	var courseID int64
	if err := database.QueryRow(`SELECT id FROM course WHERE course_code='HTML-1'`).Scan(&courseID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO entitlement(student_id,course_id,source,granted_at) VALUES(1,$1,'free','2026-01-01T00:00:00Z')`, courseID); err != nil {
		t.Fatal(err)
	}
	session, err := server.identity.CreateSession(t.Context(), 1, timeNowUTC())
	if err != nil {
		t.Fatal(err)
	}
	resources := performJSONWithCookie(t, r, http.MethodGet, "/api/v1/courses/HTML-1/lectures/1/resources", "", session.Token)
	if resources.Code != http.StatusOK || !containsAll(resources.Body.String(), `"kind":"html"`, `"inline_base_url"`) {
		t.Fatalf("导入的 HTML 资源不可读: %d %s", resources.Code, resources.Body.String())
	}
	var payload struct {
		Data struct {
			Items []struct {
				InlineBaseURL string `json:"inline_base_url"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resources.Body.Bytes(), &payload); err != nil || len(payload.Data.Items) != 1 {
		t.Fatalf("资源响应解析失败: %v %s", err, resources.Body.String())
	}
	dep := performJSONWithCookie(t, r, http.MethodGet, payload.Data.Items[0].InlineBaseURL+"assets/app.css", "", session.Token)
	if dep.Code != http.StatusOK || !strings.Contains(dep.Body.String(), "color:green") {
		t.Fatalf("HTML 依赖（CSS）未正确提供: %d %s", dep.Code, dep.Body.String())
	}
}
