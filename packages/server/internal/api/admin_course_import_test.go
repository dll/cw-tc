package api

import (
	"archive/zip"
	"bytes"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func buildCoursePackage(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
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
	return buffer.Bytes()
}

func TestAdminImportCoursePackageUpload(t *testing.T) {
	server, database, token, _ := adminDetailServer(t)
	defer database.Close()
	contentDir := t.TempDir()
	server.cfg.ContentDir = contentDir
	r := server.Router()

	var teacherID int64
	if err := database.QueryRow(`SELECT id FROM staff WHERE account='t1'`).Scan(&teacherID); err != nil {
		t.Fatal(err)
	}
	manifest := fmt.Sprintf(`{
  "course_code": "PKG-101",
  "name": "资源包导入课程",
  "line": "main",
  "price_cents": 8800,
  "teacher_id": %d,
  "lectures": [
    {"no": 1, "title": "第一讲", "is_preview": true, "assets": [
      {"title": "讲义", "path": "01/notes.md"}
    ]}
  ]
}`, teacherID)
	packageBytes := buildCoursePackage(t, map[string]string{
		"manifest.json": manifest,
		"01/notes.md":   "# 第一讲\n讲义",
	})
	upload := performMultipartWithCookie(t, r, "/api/v1/admin/courses/import-package", token,
		map[string]string{}, "course.zip", packageBytes)
	if upload.Code != http.StatusOK {
		t.Fatalf("资源包导入失败: %d %s", upload.Code, upload.Body.String())
	}
	if !containsAll(upload.Body.String(), `"course_code":"PKG-101"`, `"lectures":1`, `"assets":1`, `"status":"building"`) {
		t.Fatalf("导入结果异常: %s", upload.Body.String())
	}
	var status, source string
	if err := database.QueryRow(`SELECT status, source FROM course WHERE course_code='PKG-101'`).Scan(&status, &source); err != nil {
		t.Fatal(err)
	}
	if status != "building" || source != "platform" {
		t.Fatalf("导入课程应为 platform / building: %s %s", source, status)
	}
	if _, err := os.Stat(filepath.Join(contentDir, "courses", "PKG-101", "lectures", "1", "markdown", "notes.md")); err != nil {
		t.Fatalf("资源未落盘: %v", err)
	}
	// 重复导入 → 409
	duplicate := performMultipartWithCookie(t, r, "/api/v1/admin/courses/import-package", token,
		map[string]string{}, "course.zip", packageBytes)
	if duplicate.Code != http.StatusConflict {
		t.Fatalf("重复课程码应 409，实际 %d %s", duplicate.Code, duplicate.Body.String())
	}
}

func TestAdminImportCoursePackageRejectsBadPackage(t *testing.T) {
	server, database, token, _ := adminDetailServer(t)
	defer database.Close()
	server.cfg.ContentDir = t.TempDir()
	r := server.Router()

	cases := map[string]map[string]string{
		"缺清单":    {"01/notes.md": "# x"},
		"坏 JSON": {"manifest.json": "{"},
		"路径越界":   {"manifest.json": "{}", "../evil.txt": "x"},
	}
	for name, files := range cases {
		body := buildCoursePackage(t, files)
		upload := performMultipartWithCookie(t, r, "/api/v1/admin/courses/import-package", token,
			map[string]string{}, "course.zip", body)
		if upload.Code != http.StatusBadRequest {
			t.Fatalf("%s 应 400，实际 %d %s", name, upload.Code, upload.Body.String())
		}
	}
	notZip := performMultipartWithCookie(t, r, "/api/v1/admin/courses/import-package", token,
		map[string]string{}, "course.txt", []byte("not a zip"))
	if notZip.Code != http.StatusBadRequest {
		t.Fatalf("非 ZIP 应 400，实际 %d", notZip.Code)
	}
}

func TestAdminDeleteCourseSoftDeleteAndGuards(t *testing.T) {
	server, database, token, _ := adminDetailServer(t)
	defer database.Close()
	r := server.Router()

	if _, err := database.Exec(`INSERT INTO course(id,course_code,source,name,line,status,teacher_id,price_cents)
		VALUES(10,'DEL-1','platform','待删课程','main','building',(SELECT id FROM staff WHERE account='t1'),0)`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO course(id,course_code,source,name,line,status,price_cents)
		VALUES(11,'UP-1','upstream','上游课程','main','building',0)`); err != nil {
		t.Fatal(err)
	}

	// 必须填事由 + 二次确认
	if got := performJSONWithBearer(t, r, http.MethodDelete, "/api/v1/admin/courses/DEL-1",
		`{"confirm":true}`, token); got.Code != http.StatusBadRequest {
		t.Fatalf("缺事由应 400，实际 %d", got.Code)
	}
	// 已上架课程必须先下架
	if got := performJSONWithBearer(t, r, http.MethodDelete, "/api/v1/admin/courses/C1",
		`{"reason":"测试删除","confirm":true}`, token); got.Code != http.StatusConflict {
		t.Fatalf("已上架课程应 409，实际 %d %s", got.Code, got.Body.String())
	}
	// 上游课程不能手工删除
	if got := performJSONWithBearer(t, r, http.MethodDelete, "/api/v1/admin/courses/UP-1",
		`{"reason":"测试删除","confirm":true}`, token); got.Code != http.StatusConflict {
		t.Fatalf("上游课程应 409，实际 %d %s", got.Code, got.Body.String())
	}
	// 正常删除：软删除 + 留痕
	if got := performJSONWithBearer(t, r, http.MethodDelete, "/api/v1/admin/courses/DEL-1",
		`{"reason":"测试数据清理","confirm":true}`, token); got.Code != http.StatusOK {
		t.Fatalf("删除失败: %d %s", got.Code, got.Body.String())
	}
	var deletedAt *string
	if err := database.QueryRow(`SELECT deleted_at FROM course WHERE id=10`).Scan(&deletedAt); err != nil {
		t.Fatal(err)
	}
	if deletedAt == nil {
		t.Fatal("课程应为软删除")
	}
	var logs int
	if err := database.QueryRow(`SELECT COUNT(*) FROM course_delete_log WHERE course_code='DEL-1'`).Scan(&logs); err != nil || logs != 1 {
		t.Fatalf("删除留痕异常: %d %v", logs, err)
	}
	list := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/courses", "", token)
	if containsAll(list.Body.String(), "DEL-1") {
		t.Fatalf("已删除课程不应出现在列表: %s", list.Body.String())
	}
}
