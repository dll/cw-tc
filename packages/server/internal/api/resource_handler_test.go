package api

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cw-tc/server/internal/platform/db"
	"github.com/cw-tc/server/internal/resource"
	"github.com/cw-tc/server/migrations"
)

func TestLectureResourcesReturnsSignedURLs(t *testing.T) {
	database, err := db.Open(context.Background(), db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := db.ApplyMigrations(context.Background(), database, migrations.FS, db.DialectSQLite); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(`INSERT INTO staff(account,name,role,password_hash,created_source) VALUES('sample','样例教师','teacher','x','seed')`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(`INSERT INTO major(id,name) VALUES(1,'样例专业')`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(`INSERT INTO course(id,course_code,source,name,line,major_id,status,teacher_id,price_cents) VALUES(1,'AI-01','platform','样例课程','main',1,'published',1,1000)`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(`INSERT INTO lecture(id,course_id,no,title,is_preview) VALUES(1,1,1,'第一讲',1)`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(`INSERT INTO asset(lecture_id,kind,title,storage_key,size_bytes) VALUES(1,'slides','课件','07-CN/lectures/10/slides.pdf',1024)`); err != nil {
		t.Fatal(err)
	}
	cfg := testConfig(t, true)
	s := NewWithDatabase(cfg, database)
	response := performJSON(t, s.Router(), http.MethodGet, "/api/v1/courses/AI-01/lectures/1/resources", ``)
	body := response.Body.String()
	if response.Code != http.StatusOK || !containsAll(body, `"storage_key":"07-CN/lectures/10/slides.pdf"`, `"signature"`) {
		t.Fatalf("资源列表错误: %d %s", response.Code, body)
	}
	// 解析签名并校验可被验证
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

func TestLectureResourcesNotFound(t *testing.T) {
	database, err := db.Open(context.Background(), db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := db.ApplyMigrations(context.Background(), database, migrations.FS, db.DialectSQLite); err != nil {
		t.Fatal(err)
	}
	s := NewWithDatabase(testConfig(t, true), database)
	if resp := performJSON(t, s.Router(), http.MethodGet, "/api/v1/courses/NOPE/lectures/1/resources", ``); resp.Code != http.StatusNotFound {
		t.Fatalf("不存在讲次应返回 404: %d %s", resp.Code, resp.Body.String())
	}
}

func TestDownloadAssetSignature(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("讲义内容"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := testConfig(t, true)
	cfg.ContentDir = dir
	s := NewWithDatabase(cfg, nil)

	// 有效签名 → 200 + 文件内容
	expires := time.Now().Add(10 * time.Minute)
	sig := resource.Sign([]byte(cfg.SessionSecret), "README.md", expires)
	url := "/api/v1/assets/raw?key=README.md&expires=" + strconv.FormatInt(expires.Unix(), 10) + "&sig=" + sig
	resp := performJSON(t, s.Router(), http.MethodGet, url, ``)
	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), "讲义内容") {
		t.Fatalf("有效签名应返回文件: %d %s", resp.Code, resp.Body.String())
	}
	// 无效签名 → 403
	bad := "/api/v1/assets/raw?key=README.md&expires=" + strconv.FormatInt(expires.Unix(), 10) + "&sig=deadbeef"
	if resp := performJSON(t, s.Router(), http.MethodGet, bad, ``); resp.Code != http.StatusForbidden {
		t.Fatalf("无效签名应 403: %d", resp.Code)
	}
	// 缺参数 → 400
	if resp := performJSON(t, s.Router(), http.MethodGet, "/api/v1/assets/raw", ``); resp.Code != http.StatusBadRequest {
		t.Fatalf("缺参数应 400: %d", resp.Code)
	}
	// 目录穿越 → 403
	traversal := "/api/v1/assets/raw?key=../etc/passwd&expires=" + strconv.FormatInt(expires.Unix(), 10) + "&sig=" + resource.Sign([]byte(cfg.SessionSecret), "../etc/passwd", expires)
	if resp := performJSON(t, s.Router(), http.MethodGet, traversal, ``); resp.Code != http.StatusForbidden {
		t.Fatalf("目录穿越应 403: %d", resp.Code)
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
