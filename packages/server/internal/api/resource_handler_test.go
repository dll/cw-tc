package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cw-tc/server/internal/config"
	"github.com/cw-tc/server/internal/cosstore"
	"github.com/cw-tc/server/internal/identity"
	"github.com/cw-tc/server/internal/platform/db"
	"github.com/cw-tc/server/internal/resource"
	"github.com/cw-tc/server/migrations"
	"github.com/gin-gonic/gin"
)

var resourceDBCounter int64

// newResourceServer 构造带学员会话 + 课程/讲次/资源夹具的测试服务。
// 返回 router / cfg（可改 ContentDir）/ 学员会话 token。
func newResourceServer(t *testing.T, isPreview bool, kind, storageKey string, entitled bool) (*gin.Engine, *config.Config, string) {
	router, cfg, token, _ := newResourceServerWithDB(t, isPreview, kind, storageKey, entitled)
	return router, cfg, token
}

func newResourceServerWithDB(t *testing.T, isPreview bool, kind, storageKey string, entitled bool) (*gin.Engine, *config.Config, string, *sql.DB) {
	t.Helper()
	// 每次用独立文件库，避免共享内存库在同测试内多次调用时撞 student 唯一约束。
	dbPath := filepath.Join(t.TempDir(), fmt.Sprintf("resource_%d.db", atomic.AddInt64(&resourceDBCounter, 1)))
	database, err := db.Open(context.Background(), db.DialectSQLite, "file:"+dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := db.ApplyMigrations(context.Background(), database, migrations.FS, db.DialectSQLite); err != nil {
		t.Fatal(err)
	}
	service := identity.NewService(database, []byte("01234567890123456789012345678901"), []byte("hmac-key-for-cwTC-tests-012345678901234"))
	student, err := service.Register(context.Background(), identity.RegisterInput{Name: "样例学员", Phone: "13812345678", IDCard: "110101199001011237", Password: "password-1"})
	if err != nil {
		t.Fatal(err)
	}
	session, err := service.CreateSession(context.Background(), student.ID, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO staff(id,account,name,role,password_hash,created_source) VALUES(1,'sample','样例教师','teacher','x','seed')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO major(id,name) VALUES(1,'样例专业')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO course(id,course_code,source,name,line,major_id,status,teacher_id,price_cents) VALUES(1,'AI-01','platform','样例课程','main',1,'published',1,1000)`); err != nil {
		t.Fatal(err)
	}
	previewInt := 0
	if isPreview {
		previewInt = 1
	}
	if _, err := database.Exec(`INSERT INTO lecture(id,course_id,no,title,is_preview) VALUES(1,1,1,'第一讲',$1)`, previewInt); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO asset(lecture_id,kind,title,storage_key,size_bytes) VALUES(1,$1,'资源',$2,100)`, kind, storageKey); err != nil {
		t.Fatal(err)
	}
	if entitled {
		if _, err := database.Exec(`INSERT INTO entitlement(student_id,course_id,source,granted_at) VALUES($1,1,'free',$2)`, student.ID, time.Now().UTC().Format(time.RFC3339)); err != nil {
			t.Fatal(err)
		}
	}
	cfg := testConfig(t, true)
	s := NewWithIdentityAndDatabase(cfg, service, database)
	return s.Router(), cfg, session.Token, database
}

func TestLectureResourcesReturnsSignedURLs(t *testing.T) {
	router, cfg, token := newResourceServer(t, true, "slides", "07-CN/lectures/10/slides.pdf", false)
	response := performJSONWithCookie(t, router, http.MethodGet, "/api/v1/courses/AI-01/lectures/1/resources", "", token)
	body := response.Body.String()
	if response.Code != http.StatusOK || !containsAll(body, `"storage_key":"07-CN/lectures/10/slides.pdf"`, `"signature"`) {
		t.Fatalf("资源列表错误: %d %s", response.Code, body)
	}
	var parsed struct {
		Data struct {
			Items []struct {
				StorageKey string `json:"storage_key"`
				ExpiresAt  int64  `json:"expires_at"`
				Signature  string `json:"signature"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &parsed); err != nil || len(parsed.Data.Items) == 0 {
		t.Fatalf("解析资源响应失败: %v %s", err, body)
	}
	item := parsed.Data.Items[0]
	if !resource.Verify([]byte(cfg.SessionSecret), item.StorageKey, time.Unix(item.ExpiresAt, 0), item.Signature) {
		t.Fatal("签名应可被验证")
	}
}

func TestLectureResourcesRequiresAuth(t *testing.T) {
	router, _, _ := newResourceServer(t, true, "slides", "x.pdf", false)
	if resp := performJSON(t, router, http.MethodGet, "/api/v1/courses/AI-01/lectures/1/resources", ``); resp.Code != http.StatusUnauthorized {
		t.Fatalf("未登录应 401: %d", resp.Code)
	}
}

func TestLectureResourcesRequiresEntitlement(t *testing.T) {
	router, _, token := newResourceServer(t, false, "slides", "x.pdf", false)
	if resp := performJSONWithCookie(t, router, http.MethodGet, "/api/v1/courses/AI-01/lectures/1/resources", "", token); resp.Code != http.StatusForbidden {
		t.Fatalf("未购课非试学应 403: %d %s", resp.Code, resp.Body.String())
	}
	router2, _, token2 := newResourceServer(t, false, "slides", "x.pdf", true)
	if resp := performJSONWithCookie(t, router2, http.MethodGet, "/api/v1/courses/AI-01/lectures/1/resources", "", token2); resp.Code != http.StatusOK {
		t.Fatalf("已购课应 200: %d", resp.Code)
	}
}

func TestDownloadAssetSignature(t *testing.T) {
	router, cfg, token := newResourceServer(t, true, "markdown", "README.md", false)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("讲义内容"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg.ContentDir = dir

	expires := time.Now().Add(10 * time.Minute)
	sig := resource.Sign([]byte(cfg.SessionSecret), "README.md", expires)
	url := "/api/v1/assets/raw?key=README.md&expires=" + strconv.FormatInt(expires.Unix(), 10) + "&sig=" + sig
	resp := performJSONWithCookie(t, router, http.MethodGet, url, "", token)
	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), "讲义内容") {
		t.Fatalf("有效签名应返回文件: %d %s", resp.Code, resp.Body.String())
	}
	if got := resp.Header().Get("Content-Disposition"); got != "inline" {
		t.Fatalf("非源码应为 inline，实际 %q", got)
	}
	bad := "/api/v1/assets/raw?key=README.md&expires=" + strconv.FormatInt(expires.Unix(), 10) + "&sig=deadbeef"
	if resp := performJSONWithCookie(t, router, http.MethodGet, bad, "", token); resp.Code != http.StatusForbidden {
		t.Fatalf("无效签名应 403: %d", resp.Code)
	}
}

func TestDownloadAssetCodeAttachment(t *testing.T) {
	router, cfg, token := newResourceServer(t, true, "code", "main.c", false)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.c"), []byte("int main(){}"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg.ContentDir = dir

	expires := time.Now().Add(10 * time.Minute)
	sig := resource.Sign([]byte(cfg.SessionSecret), "main.c", expires)
	url := "/api/v1/assets/raw?key=main.c&expires=" + strconv.FormatInt(expires.Unix(), 10) + "&sig=" + sig
	resp := performJSONWithCookie(t, router, http.MethodGet, url, "", token)
	if resp.Code != http.StatusOK {
		t.Fatalf("源码应可下载: %d %s", resp.Code, resp.Body.String())
	}
	if got := resp.Header().Get("Content-Disposition"); !strings.HasPrefix(got, "attachment; filename=\"main.c\"") {
		t.Fatalf("源码应为 attachment，实际 %q", got)
	}
}

func TestDownloadAssetRequiresEntitlement(t *testing.T) {
	router, cfg, token := newResourceServer(t, false, "markdown", "README.md", false)
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "README.md"), []byte("x"), 0o644)
	cfg.ContentDir = dir

	expires := time.Now().Add(10 * time.Minute)
	sig := resource.Sign([]byte(cfg.SessionSecret), "README.md", expires)
	url := "/api/v1/assets/raw?key=README.md&expires=" + strconv.FormatInt(expires.Unix(), 10) + "&sig=" + sig
	if resp := performJSONWithCookie(t, router, http.MethodGet, url, "", token); resp.Code != http.StatusForbidden {
		t.Fatalf("未购课非试学下载应 403: %d", resp.Code)
	}
}

func TestDownloadAssetNotFound(t *testing.T) {
	router, cfg, token := newResourceServer(t, true, "markdown", "README.md", false)
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "README.md"), []byte("x"), 0o644)
	cfg.ContentDir = dir

	expires := time.Now().Add(10 * time.Minute)
	sig := resource.Sign([]byte(cfg.SessionSecret), "nope.txt", expires)
	url := "/api/v1/assets/raw?key=nope.txt&expires=" + strconv.FormatInt(expires.Unix(), 10) + "&sig=" + sig
	if resp := performJSONWithCookie(t, router, http.MethodGet, url, "", token); resp.Code != http.StatusNotFound {
		t.Fatalf("不存在资源应 404: %d", resp.Code)
	}
}

func TestLectureResourcesFailClosedWithoutDB(t *testing.T) {
	database, err := db.Open(context.Background(), db.DialectSQLite, "file:"+filepath.Join(t.TempDir(), "x.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := db.ApplyMigrations(context.Background(), database, migrations.FS, db.DialectSQLite); err != nil {
		t.Fatal(err)
	}
	service := identity.NewService(database, []byte("01234567890123456789012345678901"), []byte("hmac-key-for-cwTC-tests-012345678901234"))
	student, _ := service.Register(context.Background(), identity.RegisterInput{Name: "样例学员", Phone: "13812345678", IDCard: "110101199001011237", Password: "password-1"})
	session, _ := service.CreateSession(context.Background(), student.ID, time.Now().UTC())
	s := NewWithIdentity(testConfig(t, true), service)
	if resp := performJSONWithCookie(t, s.Router(), http.MethodGet, "/api/v1/courses/AI-01/lectures/1/resources", "", session.Token); resp.Code != http.StatusServiceUnavailable {
		t.Fatalf("无 db 应 503: %d", resp.Code)
	}
}

func TestIsCosBackedKey(t *testing.T) {
	for _, k := range []string{"a/讲解.mp4", "b.mov", "c.mp3", "d.wav", "e.MP4", "f.m4a"} {
		if !isCosBackedKey(k) {
			t.Errorf("isCosBackedKey(%q) 应为 true", k)
		}
	}
	for _, k := range []string{"a/README.md", "b.pptx", "c.html", "d.c", "e.pdf", ""} {
		if isCosBackedKey(k) {
			t.Errorf("isCosBackedKey(%q) 应为 false", k)
		}
	}
}

// TestLectureResourcesVideoFirst 验证视频资源始终排在清单第一位（即使 sort 不是最小）。
func TestLectureResourcesVideoFirst(t *testing.T) {
	database, err := db.Open(context.Background(), db.DialectSQLite, "file:"+filepath.Join(t.TempDir(), "order.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := db.ApplyMigrations(context.Background(), database, migrations.FS, db.DialectSQLite); err != nil {
		t.Fatal(err)
	}
	service := identity.NewService(database, []byte("01234567890123456789012345678901"), []byte("hmac-key-for-cwTC-tests-012345678901234"))
	student, err := service.Register(context.Background(), identity.RegisterInput{Name: "样例学员", Phone: "13812345679", IDCard: "110101199001011237", Password: "password-1"})
	if err != nil {
		t.Fatal(err)
	}
	session, err := service.CreateSession(context.Background(), student.ID, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`INSERT INTO staff(id,account,name,role,password_hash,created_source) VALUES(1,'sample','样例教师','teacher','x','seed')`,
		`INSERT INTO major(id,name) VALUES(1,'样例专业')`,
		`INSERT INTO course(id,course_code,source,name,line,major_id,status,teacher_id,price_cents) VALUES(1,'AI-01','platform','样例课程','main',1,'published',1,1000)`,
		`INSERT INTO lecture(id,course_id,no,title,is_preview) VALUES(1,1,1,'第一讲',0)`,
		// 视频 sort=5 不是最小，但应排第一；markdown/code sort 更小。
		`INSERT INTO asset(lecture_id,kind,title,storage_key,size_bytes,sort) VALUES(1,'markdown','讲义','x.md',100,1)`,
		`INSERT INTO asset(lecture_id,kind,title,storage_key,size_bytes,sort) VALUES(1,'video','视频','x.mp4',100,5)`,
		`INSERT INTO asset(lecture_id,kind,title,storage_key,size_bytes,sort) VALUES(1,'code','代码','x.go',100,2)`,
		fmt.Sprintf(`INSERT INTO entitlement(student_id,course_id,source,granted_at) VALUES(%d,1,'free','%s')`, student.ID, time.Now().UTC().Format(time.RFC3339)),
	} {
		if _, err := database.Exec(stmt); err != nil {
			t.Fatalf("夹具失败: %v\n%s", err, stmt)
		}
	}
	s := NewWithIdentityAndDatabase(testConfig(t, true), service, database)
	resp := performJSONWithCookie(t, s.Router(), http.MethodGet, "/api/v1/courses/AI-01/lectures/1/resources", "", session.Token)
	var body struct {
		Data struct {
			Items []struct {
				Kind string `json:"kind"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("解析响应失败: %v\n%s", err, resp.Body.String())
	}
	if len(body.Data.Items) != 3 {
		t.Fatalf("应返回 3 个资源，实际 %d: %s", len(body.Data.Items), resp.Body.String())
	}
	if body.Data.Items[0].Kind != "video" {
		t.Fatalf("视频应排第一位，实际第一是 %q", body.Data.Items[0].Kind)
	}
}

// TestFreeCourseAccess 免费课（price_cents=0）无需购买即可访问资源；付费课未购仍 403。
func TestFreeCourseAccess(t *testing.T) {
	database, err := db.Open(context.Background(), db.DialectSQLite, "file:"+filepath.Join(t.TempDir(), "free.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := db.ApplyMigrations(context.Background(), database, migrations.FS, db.DialectSQLite); err != nil {
		t.Fatal(err)
	}
	service := identity.NewService(database, []byte("01234567890123456789012345678901"), []byte("hmac-key-for-cwTC-tests-012345678901234"))
	student, err := service.Register(context.Background(), identity.RegisterInput{Name: "样例学员", Phone: "13812345679", IDCard: "110101199001011237", Password: "password-1"})
	if err != nil {
		t.Fatal(err)
	}
	session, err := service.CreateSession(context.Background(), student.ID, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`INSERT INTO staff(id,account,name,role,password_hash,created_source) VALUES(1,'sample','样例教师','teacher','x','seed')`,
		`INSERT INTO major(id,name) VALUES(1,'样例专业')`,
		// 免费课 price_cents=0；付费课 price_cents=1000
		`INSERT INTO course(id,course_code,source,name,line,major_id,status,teacher_id,price_cents) VALUES(1,'FREE-01','platform','免费课','main',1,'published',1,0),(2,'PAID-01','platform','付费课','main',1,'published',1,1000)`,
		`INSERT INTO lecture(id,course_id,no,title,is_preview) VALUES(1,1,1,'第一讲',0),(2,2,1,'第一讲',0)`,
		`INSERT INTO asset(lecture_id,kind,title,storage_key,size_bytes) VALUES(1,'markdown','讲义','free.md',100),(2,'markdown','讲义','paid.md',100)`,
	} {
		if _, err := database.Exec(stmt); err != nil {
			t.Fatalf("夹具失败: %v\n%s", err, stmt)
		}
	}
	s := NewWithIdentityAndDatabase(testConfig(t, true), service, database)
	// 免费课未购 → 200
	if resp := performJSONWithCookie(t, s.Router(), http.MethodGet, "/api/v1/courses/FREE-01/lectures/1/resources", "", session.Token); resp.Code != http.StatusOK {
		t.Fatalf("免费课未购应 200，实际 %d: %s", resp.Code, resp.Body.String())
	}
	// 付费课未购 → 403
	if resp := performJSONWithCookie(t, s.Router(), http.MethodGet, "/api/v1/courses/PAID-01/lectures/1/resources", "", session.Token); resp.Code != http.StatusForbidden {
		t.Fatalf("付费课未购应 403，实际 %d: %s", resp.Code, resp.Body.String())
	}
}

// TestLectureResourcesMediaForceFile 验证 MediaForceFile 时（COS 欠费/停服临时回退），
// 视频项下发文件服务签名 URL（/api/v1/assets/raw?），而非 COS 预签名 URL。
func TestLectureResourcesMediaForceFile(t *testing.T) {
	router, cfg, token := newResourceServer(t, true, "video", "AI化教学创新02-DO/01-案例A/video/讲解.mp4", false)
	cfg.MediaForceFile = true // 模拟 COS 欠费回退
	resp := performJSONWithCookie(t, router, http.MethodGet, "/api/v1/courses/AI-01/lectures/1/resources", "", token)
	if resp.Code != http.StatusOK {
		t.Fatalf("资源清单应 200，实际 %d: %s", resp.Code, resp.Body.String())
	}
	var body struct {
		Data struct {
			Items []struct {
				Kind string `json:"kind"`
				URL  string `json:"url"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("响应解析失败: %v", err)
	}
	if len(body.Data.Items) != 1 || body.Data.Items[0].URL == "" {
		t.Fatalf("视频项应带 url: %s", resp.Body.String())
	}
	if !strings.HasPrefix(body.Data.Items[0].URL, "/api/v1/assets/media?") {
		t.Fatalf("MediaForceFile 应下发文件服务 URL，实际 %q", body.Data.Items[0].URL)
	}
	if strings.Contains(body.Data.Items[0].URL, "myqcloud.com") {
		t.Fatalf("MediaForceFile 不应下发 COS URL: %q", body.Data.Items[0].URL)
	}
}

func TestSignedMediaStreamsWithoutCookie(t *testing.T) {
	key := "course/video/sample.mp4"
	router, cfg, token := newResourceServer(t, true, "video", key, false)
	cfg.MediaForceFile = true
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "course", "video"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(key)), []byte("sample-video-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg.ContentDir = dir
	listed := performJSONWithCookie(t, router, http.MethodGet, "/api/v1/courses/AI-01/lectures/1/resources", "", token)
	var body struct {
		Data struct {
			Items []struct {
				URL string `json:"url"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(listed.Body.Bytes(), &body); err != nil || len(body.Data.Items) != 1 {
		t.Fatalf("资源清单异常: %v %s", err, listed.Body.String())
	}
	u := body.Data.Items[0].URL
	req := httptest.NewRequest(http.MethodGet, u, nil)
	req.Header.Set("Range", "bytes=0-5")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusPartialContent || resp.Body.String() != "sample" || !strings.HasPrefix(resp.Header().Get("Content-Type"), "video/mp4") {
		t.Fatalf("无 Cookie 的视频 Range 请求异常: %d %s %s", resp.Code, resp.Header().Get("Content-Type"), resp.Body.String())
	}
	parsed, _ := url.Parse(u)
	q := parsed.Query()
	q.Set("student", "999")
	parsed.RawQuery = q.Encode()
	if bad := performJSON(t, router, http.MethodGet, parsed.String(), ""); bad.Code != http.StatusForbidden {
		t.Fatalf("篡改学员后应 403，实际 %d", bad.Code)
	}
}

func TestHtmlPreviewAndInlineAudio(t *testing.T) {
	router, cfg, token, database := newResourceServerWithDB(t, false, "html", "lesson/page.html", false)
	if _, err := database.Exec(`UPDATE course SET free_preview_lectures = 2 WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO asset(lecture_id,kind,title,storage_key,size_bytes) VALUES(1,'audio','讲解','lesson/assets/audio/step_00.mp3',16)`); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "lesson", "assets", "audio"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "lesson", "page.html"), []byte("<html>课件</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "lesson", "assets", "audio", "step_00.mp3"), []byte("audio-sample"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "lesson", "assets", "avatar.jpg"), []byte("image-sample"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg.ContentDir = dir
	cfg.MediaForceFile = true
	listed := performJSONWithCookie(t, router, http.MethodGet, "/api/v1/courses/AI-01/lectures/1/resources", "", token)
	if listed.Code != http.StatusOK {
		t.Fatalf("试学资源清单应 200: %d %s", listed.Code, listed.Body.String())
	}
	var body struct {
		Data struct {
			Items []struct {
				Kind          string `json:"kind"`
				InlineBaseURL string `json:"inline_base_url"`
				StorageKey    string `json:"storage_key"`
				ExpiresAt     int64  `json:"expires_at"`
				Signature     string `json:"signature"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(listed.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	var htmlItem *struct {
		Kind          string `json:"kind"`
		InlineBaseURL string `json:"inline_base_url"`
		StorageKey    string `json:"storage_key"`
		ExpiresAt     int64  `json:"expires_at"`
		Signature     string `json:"signature"`
	}
	for i := range body.Data.Items {
		if body.Data.Items[i].Kind == "html" {
			htmlItem = &body.Data.Items[i]
		}
	}
	if htmlItem == nil || htmlItem.InlineBaseURL == "" {
		t.Fatalf("HTML 应下发内嵌资源基址: %s", listed.Body.String())
	}
	rawURL := "/api/v1/assets/raw?key=lesson%2Fpage.html&expires=" + strconv.FormatInt(htmlItem.ExpiresAt, 10) + "&sig=" + htmlItem.Signature
	if raw := performJSONWithCookie(t, router, http.MethodGet, rawURL, "", token); raw.Code != http.StatusOK {
		t.Fatalf("平台试学讲次的 HTML 下载应 200: %d %s", raw.Code, raw.Body.String())
	}
	audioURL := htmlItem.InlineBaseURL + "assets/audio/step_00.mp3"
	req := httptest.NewRequest(http.MethodGet, audioURL, nil)
	req.Header.Set("Range", "bytes=0-4")
	audio := httptest.NewRecorder()
	router.ServeHTTP(audio, req)
	if audio.Code != http.StatusPartialContent || audio.Body.String() != "audio" || !strings.HasPrefix(audio.Header().Get("Content-Type"), "audio/mpeg") {
		t.Fatalf("HTML 内相对音频应无需 Cookie 可播放: %d %s %s", audio.Code, audio.Header().Get("Content-Type"), audio.Body.String())
	}
	if missing := performJSON(t, router, http.MethodGet, htmlItem.InlineBaseURL+"assets/audio/secret.mp3", ""); missing.Code != http.StatusNotFound {
		t.Fatalf("未入库的同目录文件应 404: %d", missing.Code)
	}
	if picture := performJSON(t, router, http.MethodGet, htmlItem.InlineBaseURL+"assets/avatar.jpg", ""); picture.Code != http.StatusOK || picture.Body.String() != "image-sample" {
		t.Fatalf("HTML 内未入库的配图应可加载: %d %s", picture.Code, picture.Body.String())
	}
	if _, err := database.Exec(`UPDATE course SET free_preview_lectures = 0 WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
	if denied := performJSON(t, router, http.MethodGet, audioURL, ""); denied.Code != http.StatusForbidden {
		t.Fatalf("撤销试学后签名音频应 403: %d", denied.Code)
	}
}

// TestCosAvailable 覆盖 COS 可用性探测的短路与缓存路径。
func TestCosAvailable(t *testing.T) {
	// cosStore 未配置 → false
	s := New(testConfig(t, true))
	if s.cosAvailable(context.Background(), "a.mp4") {
		t.Fatal("cosStore 未配置应返回 false")
	}
	// MediaForceFile → false
	cfg := testConfig(t, true)
	cfg.MediaForceFile = true
	s2 := New(cfg)
	if s2.cosAvailable(context.Background(), "a.mp4") {
		t.Fatal("MediaForceFile 应返回 false")
	}
	// 缓存未过期 → 直接返回缓存值（不触发真实探测）
	s3 := New(testConfig(t, true))
	s3.cosStore = cosstore.New("id", "k", "b", "ap-nanjing")
	s3.cosProbes = map[string]cosProbeResult{"a.mp4": {at: time.Now(), ok: true}}
	if !s3.cosAvailable(context.Background(), "a.mp4") {
		t.Fatal("缓存命中应返回 true")
	}
	s3.cosProbes["a.mp4"] = cosProbeResult{at: time.Now(), ok: false}
	if s3.cosAvailable(context.Background(), "a.mp4") {
		t.Fatal("缓存命中应返回 false")
	}
	// 缓存过期 → 触发真实探测（假凭据，COS 返回 403/网络错误 → false，并更新缓存时间戳）
	s4 := New(testConfig(t, true))
	s4.cosStore = cosstore.New("id", "k", "b", "ap-nanjing")
	s4.cosProbes = map[string]cosProbeResult{"a.mp4": {at: time.Now().Add(-10 * time.Minute), ok: true}}
	if s4.cosAvailable(context.Background(), "a.mp4") {
		t.Fatal("假凭据探测应返回 false")
	}
	if time.Since(s4.cosProbes["a.mp4"].at) >= 5*time.Minute {
		t.Fatal("探测后应更新缓存时间戳")
	}
}
