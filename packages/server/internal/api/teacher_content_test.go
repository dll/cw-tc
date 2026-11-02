package api

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/cw-tc/server/internal/staff"
	"github.com/gin-gonic/gin"
)

func performMultipartWithCookie(t *testing.T, r *gin.Engine, path, token string, fields map[string]string, fileName string, content []byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range fields {
		_ = writer.WriteField(key, value)
	}
	part, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatal(err)
	}
	_ = writer.Close()
	request := httptest.NewRequest(http.MethodPost, path, &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	r.ServeHTTP(response, request)
	return response
}

func teacherToken(t *testing.T, r *gin.Engine, account string) string {
	t.Helper()
	login := performJSON(t, r, http.MethodPost, "/api/v1/admin/login", `{"account":"`+account+`","password":"Temp@123"}`)
	return extractJSONField(t, login.Body.String(), "token")
}

func TestTeacherCourseContentAndUpload(t *testing.T) {
	server, database, _, _ := adminDetailServer(t)
	defer database.Close()
	contentDir := t.TempDir()
	server.cfg.ContentDir = contentDir
	r := server.Router()
	token := teacherToken(t, r, "t1")

	create := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/teacher/courses",
		`{"course_code":"TC-1","name":"教师自建课","line":"main"}`, token)
	if create.Code != http.StatusOK {
		t.Fatalf("创建课程失败: %d %s", create.Code, create.Body.String())
	}
	empty := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/teacher/courses/TC-1/content", "", token)
	if empty.Code != http.StatusOK || !containsAll(empty.Body.String(), `"editable":true`, `"lectures":[]`) {
		t.Fatalf("内容读取异常: %d %s", empty.Code, empty.Body.String())
	}
	lecture := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/teacher/courses/TC-1/lectures",
		`{"title":"第一讲","is_preview":true}`, token)
	if lecture.Code != http.StatusOK {
		t.Fatalf("新增讲次失败: %d %s", lecture.Code, lecture.Body.String())
	}
	var lectureCount, version int
	if err := database.QueryRow(`SELECT lecture_count, content_version FROM course WHERE course_code='TC-1'`).Scan(&lectureCount, &version); err != nil || lectureCount != 1 || version < 2 {
		t.Fatalf("课程计数 / 版本未更新: %d %d %v", lectureCount, version, err)
	}

	upload := performMultipartWithCookie(t, r, "/api/v1/teacher/courses/TC-1/lectures/1/assets", token,
		map[string]string{"kind": "video", "title": "视频"}, "video.mp4", []byte("fake-mp4-content"))
	if upload.Code != http.StatusOK || !containsAll(upload.Body.String(), `"kind":"video"`, `"storage_key":"courses/TC-1/lectures/1/video/video.mp4"`) {
		t.Fatalf("上传视频失败: %d %s", upload.Code, upload.Body.String())
	}
	if info, err := os.Stat(filepath.Join(contentDir, "courses", "TC-1", "lectures", "1", "video", "video.mp4")); err != nil || info.Size() == 0 {
		t.Fatalf("视频未落盘: %v", err)
	}
	html := performMultipartWithCookie(t, r, "/api/v1/teacher/courses/TC-1/lectures/1/assets", token,
		map[string]string{"title": "课件"}, "slides.html", []byte("<html>self-contained</html>"))
	if html.Code != http.StatusOK || !containsAll(html.Body.String(), `"kind":"html"`) {
		t.Fatalf("上传 HTML 失败: %d %s", html.Code, html.Body.String())
	}
	content := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/teacher/courses/TC-1/content", "", token)
	if content.Code != http.StatusOK || !containsAll(content.Body.String(), `"kind":"video"`, `"kind":"html"`, `"no":1`) {
		t.Fatalf("内容清单异常: %d %s", content.Code, content.Body.String())
	}

	var assetID int64
	if err := database.QueryRow(`SELECT id FROM asset WHERE storage_key='courses/TC-1/lectures/1/html/slides.html'`).Scan(&assetID); err != nil {
		t.Fatal(err)
	}
	if got := performJSONWithBearer(t, r, http.MethodDelete,
		"/api/v1/teacher/courses/TC-1/lectures/1/assets/"+itoaTest(assetID), "", token); got.Code != http.StatusOK {
		t.Fatalf("删除资源失败: %d %s", got.Code, got.Body.String())
	}
	if _, err := os.Stat(filepath.Join(contentDir, "courses", "TC-1", "lectures", "1", "html", "slides.html")); !os.IsNotExist(err) {
		t.Fatalf("删除资源应清理文件: %v", err)
	}

	// 已上架课程不可再改内容（需走版本 / 审核）。
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/teacher/courses/C1/lectures", `{"title":"绕过"}`, token); got.Code != http.StatusConflict {
		t.Fatalf("已上架课程新增讲次应 409，实际 %d", got.Code)
	}
	published := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/teacher/courses/C1/content", "", token)
	if published.Code != http.StatusOK || !containsAll(published.Body.String(), `"editable":false`) {
		t.Fatalf("已上架课程应为只读: %d %s", published.Code, published.Body.String())
	}
}

func TestTeacherContentOwnership(t *testing.T) {
	server, database, _, _ := adminDetailServer(t)
	defer database.Close()
	if _, err := staff.Create(context.Background(), database, "t2", "教师二", staff.RoleTeacher, "Temp@123", 0); err != nil {
		t.Fatal(err)
	}
	server.cfg.ContentDir = t.TempDir()
	r := server.Router()
	token := teacherToken(t, r, "t2")
	if got := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/teacher/courses/C1/content", "", token); got.Code != http.StatusForbidden {
		t.Fatalf("非本人课程应 403，实际 %d", got.Code)
	}
}

func itoaTest(v int64) string {
	if v == 0 {
		return "0"
	}
	digits := make([]byte, 0, 20)
	for v > 0 {
		digits = append([]byte{byte('0' + v%10)}, digits...)
		v /= 10
	}
	return string(digits)
}

func TestTeacherContentErrorBranches(t *testing.T) {
	server, database, _, _ := adminDetailServer(t)
	defer database.Close()
	server.cfg.ContentDir = t.TempDir()
	r := server.Router()
	token := teacherToken(t, r, "t1")
	if _, err := staff.Create(context.Background(), database, "t2", "教师二", staff.RoleTeacher, "Temp@123", 0); err != nil {
		t.Fatal(err)
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/teacher/courses",
		`{"course_code":"TC-ERR","name":"错误用例课"}`, token); got.Code != http.StatusOK {
		t.Fatalf("创建课程失败: %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/teacher/courses/NOPE/content", "", token); got.Code != http.StatusNotFound {
		t.Fatalf("不存在课程应 404，实际 %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/teacher/courses/TC-ERR/lectures", `{"title":""}`, token); got.Code != http.StatusBadRequest {
		t.Fatalf("空标题应 400，实际 %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/teacher/courses/TC-ERR/lectures", `{"title":"第一讲"}`, token); got.Code != http.StatusOK {
		t.Fatalf("新增讲次失败: %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/teacher/courses/TC-ERR/lectures/1/assets", `{}`, token); got.Code != http.StatusBadRequest {
		t.Fatalf("非 multipart 上传应 400，实际 %d", got.Code)
	}
	if got := performMultipartWithCookie(t, r, "/api/v1/teacher/courses/TC-ERR/lectures/1/assets", token,
		map[string]string{"title": "未知"}, "file.exe", []byte("x")); got.Code != http.StatusBadRequest {
		t.Fatalf("未知资源类型应 400，实际 %d", got.Code)
	}
	if got := performMultipartWithCookie(t, r, "/api/v1/teacher/courses/TC-ERR/lectures/1/assets", token,
		map[string]string{"title": "讲义"}, "notes.md", []byte("# 讲义")); got.Code != http.StatusOK {
		t.Fatalf("上传讲义失败: %d %s", got.Code, got.Body.String())
	}
	if got := performMultipartWithCookie(t, r, "/api/v1/teacher/courses/TC-ERR/lectures/1/assets", token,
		map[string]string{"title": "重复"}, "notes.md", []byte("# 讲义")); got.Code != http.StatusConflict {
		t.Fatalf("同名资源应 409，实际 %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodDelete, "/api/v1/teacher/courses/TC-ERR/lectures/1/assets/999", "", token); got.Code != http.StatusNotFound {
		t.Fatalf("删除不存在资源应 404，实际 %d", got.Code)
	}
	other := teacherToken(t, r, "t2")
	if got := performMultipartWithCookie(t, r, "/api/v1/teacher/courses/TC-ERR/lectures/1/assets", other,
		map[string]string{"title": "越权"}, "x.md", []byte("x")); got.Code != http.StatusForbidden {
		t.Fatalf("非本人课程上传应 403，实际 %d", got.Code)
	}
}

func TestTeacherDiscussionErrors(t *testing.T) {
	server, database, _, _ := adminDetailServer(t)
	defer database.Close()
	r := server.Router()
	token := teacherToken(t, r, "t1")
	if got := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/teacher/discussions?limit=0", "", token); got.Code != http.StatusBadRequest {
		t.Fatalf("非法 limit 应 400，实际 %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/teacher/discussions/bad/replies", `{"body":"回复"}`, token); got.Code != http.StatusBadRequest {
		t.Fatalf("非法帖子号应 400，实际 %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/teacher/discussions/999/replies", `{"body":"回复内容"}`, token); got.Code != http.StatusNotFound {
		t.Fatalf("不存在帖子应 404，实际 %d", got.Code)
	}
}
