package api

import (
	"context"
	"encoding/json"
	"net/http"
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
