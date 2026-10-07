package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/cw-tc/server/internal/platform/db"
	"github.com/cw-tc/server/migrations"
)

func catalogServer(t *testing.T) (*Server, func()) {
	t.Helper()
	database, err := db.Open(context.Background(), db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.ApplyMigrations(context.Background(), database, migrations.FS, db.DialectSQLite); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(`INSERT INTO staff(account,name,role,password_hash,created_source) VALUES('sample','样例教师','teacher','x','seed')`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(`INSERT INTO major(id,name,sort,status) VALUES(1,'人工智能',0,'enabled'),(2,'网络安全',1,'enabled'),(3,'已停用',2,'disabled')`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(`INSERT INTO course(id,course_code,source,name,line,major_id,status,teacher_id,price_cents) VALUES(1,'AI-01','platform','人工智能基础','main',1,'published',1,1000),(2,'SEC-01','platform','网络安全基础','main',2,'published',1,2000),(3,'HIDDEN','platform','未上架','main',1,'building',1,3000)`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(`INSERT INTO lecture(course_id,no,title,is_preview) VALUES(1,1,'第一讲',1),(1,2,'第二讲',0),(1,3,'第三讲',0)`); err != nil {
		t.Fatal(err)
	}
	return NewWithDatabase(testConfig(t, true), database), func() { database.Close() }
}

func TestMajorListReturnsOnlyEnabled(t *testing.T) {
	s, cleanup := catalogServer(t)
	defer cleanup()
	response := performJSON(t, s.Router(), http.MethodGet, "/api/v1/majors", ``)
	if response.Code != http.StatusOK || !containsAll(response.Body.String(), `"name":"人工智能"`, `"name":"网络安全"`) || containsAll(response.Body.String(), "已停用") {
		t.Fatalf("专业列表错误: %d %s", response.Code, response.Body.String())
	}
}

func TestCourseListFiltersByMajor(t *testing.T) {
	s, cleanup := catalogServer(t)
	defer cleanup()
	response := performJSON(t, s.Router(), http.MethodGet, "/api/v1/courses?major_id=1", ``)
	if response.Code != http.StatusOK || !containsAll(response.Body.String(), "AI-01") || containsAll(response.Body.String(), "SEC-01") {
		t.Fatalf("按专业筛选失败: %d %s", response.Code, response.Body.String())
	}
}

func TestLectureListReturnsOrderedWithPreview(t *testing.T) {
	s, cleanup := catalogServer(t)
	defer cleanup()
	response := performJSON(t, s.Router(), http.MethodGet, "/api/v1/courses/AI-01/lectures", ``)
	body := response.Body.String()
	if response.Code != http.StatusOK || !containsAll(body, `"no":1`, `"no":3`, `"is_preview":true`) {
		t.Fatalf("讲次列表错误: %d %s", response.Code, body)
	}
	if body != "" && (indexOf(body, `"no":1`) == -1 || indexOf(body, `"no":1`) > indexOf(body, `"no":3`)) {
		t.Fatalf("讲次应有序: %s", body)
	}
}

func indexOf(value, needle string) int {
	for i := 0; i+len(needle) <= len(value); i++ {
		if value[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
