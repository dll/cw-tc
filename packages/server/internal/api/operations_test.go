package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/cw-tc/server/internal/platform/db"
	"github.com/cw-tc/server/internal/staff"
	"github.com/cw-tc/server/migrations"
)

func TestAdminOperationsFlow(t *testing.T) {
	ctx := t.Context()
	database, err := db.Open(ctx, db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := db.ApplyMigrations(ctx, database, migrations.FS, db.DialectSQLite); err != nil {
		t.Fatal(err)
	}
	if _, err := staff.Create(ctx, database, "admin", "管理员", staff.RoleAdminOps, "Temp@123", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := staff.Create(ctx, database, "teacher", "教师", staff.RoleTeacher, "Temp@123", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO major(id,name) VALUES(1,'样例专业')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO course(course_code,source,name,line,major_id,status,teacher_id,cert_names,cert_pass_lines) VALUES('C1','platform','课程','main',1,'published',2,'{"L1":"证书"}','{"L1":60}')`); err != nil {
		t.Fatal(err)
	}
	r := NewWithDatabase(testConfig(t, true), database).Router()
	adminToken := extractJSONField(t, performJSON(t, r, http.MethodPost, "/api/v1/admin/login", `{"account":"admin","password":"Temp@123"}`).Body.String(), "token")
	teacherToken := extractJSONField(t, performJSON(t, r, http.MethodPost, "/api/v1/admin/login", `{"account":"teacher","password":"Temp@123"}`).Body.String(), "token")
	if got := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/operations", "", teacherToken); got.Code != http.StatusForbidden {
		t.Fatalf("教师读取运营位应 403: %d", got.Code)
	}
	now := time.Now().UTC()
	payload := `{"position":"home_banner","course_code":"C1","title":"新学期推荐","image_url":"/media/banner.png","link_url":"","starts_at":"` +
		now.Add(-time.Hour).Format(time.RFC3339) + `","ends_at":"` + now.Add(time.Hour).Format(time.RFC3339) + `","weight":10,"status":"active"}`
	create := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/operations", payload, adminToken)
	if create.Code != http.StatusOK {
		t.Fatalf("创建运营位失败: %d %s", create.Code, create.Body.String())
	}
	var created struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(create.Body.Bytes(), &created); err != nil || created.Data.ID <= 0 {
		t.Fatalf("运营位 id 异常: %s", create.Body.String())
	}
	list := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/operations", "", adminToken)
	if list.Code != http.StatusOK || !containsAll(list.Body.String(), `"title":"新学期推荐"`) {
		t.Fatalf("运营位列表异常: %d %s", list.Code, list.Body.String())
	}
	public := performJSON(t, r, http.MethodGet, "/api/v1/public/operations?position=home_banner", "")
	if public.Code != http.StatusOK || !containsAll(public.Body.String(), `"title":"新学期推荐"`) {
		t.Fatalf("公开运营位异常: %d %s", public.Code, public.Body.String())
	}
	if got := performJSON(t, r, http.MethodGet, "/api/v1/public/operations?position=bad", ""); got.Code != http.StatusBadRequest {
		t.Fatalf("非法 position 应 400: %d", got.Code)
	}
	updatePayload := `{"position":"home_banner","course_code":"C1","title":"更新后的推荐","image_url":"/media/banner.png","link_url":"","starts_at":"` +
		now.Add(-time.Hour).Format(time.RFC3339) + `","ends_at":"` + now.Add(time.Hour).Format(time.RFC3339) + `","weight":20,"status":"active"}`
	update := performJSONWithBearer(t, r, http.MethodPatch, "/api/v1/admin/operations/"+strconv.FormatInt(created.Data.ID, 10), updatePayload, adminToken)
	if update.Code != http.StatusOK {
		t.Fatalf("更新运营位失败: %d %s", update.Code, update.Body.String())
	}
	if got := performJSONWithBearer(t, r, http.MethodDelete, "/api/v1/admin/operations/"+strconv.FormatInt(created.Data.ID, 10), "", adminToken); got.Code != http.StatusOK {
		t.Fatalf("下线运营位失败: %d", got.Code)
	}
	public = performJSON(t, r, http.MethodGet, "/api/v1/public/operations?position=home_banner", "")
	if public.Code != http.StatusOK || containsAll(public.Body.String(), `"title":"更新后的推荐"`) {
		t.Fatalf("下线后公开运营位应为空: %s", public.Body.String())
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/operations", `{"position":"bad","title":"x","starts_at":"2026-01-01T00:00:00Z","ends_at":"2026-01-02T00:00:00Z","status":"active"}`, adminToken); got.Code != http.StatusBadRequest {
		t.Fatalf("非法位置应 400: %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodPatch, "/api/v1/admin/operations/999", updatePayload, adminToken); got.Code != http.StatusNotFound {
		t.Fatalf("更新不存在运营位应 404: %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodDelete, "/api/v1/admin/operations/999", "", adminToken); got.Code != http.StatusNotFound {
		t.Fatalf("下线不存在运营位应 404: %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/operations", `{"position":"home_banner","title":"x","starts_at":"2026-01-02T00:00:00Z","ends_at":"2026-01-01T00:00:00Z","status":"active"}`, adminToken); got.Code != http.StatusBadRequest {
		t.Fatalf("结束早于开始应 400: %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/operations", `{"position":"home_banner","starts_at":"2026-01-01T00:00:00Z","ends_at":"2026-01-02T00:00:00Z","status":"active"}`, adminToken); got.Code != http.StatusBadRequest {
		t.Fatalf("缺标题与图片应 400: %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/operations", `{"position":"home_recommend","title":"x","starts_at":"2026-01-01T00:00:00Z","ends_at":"2026-01-02T00:00:00Z","status":"active"}`, adminToken); got.Code != http.StatusBadRequest {
		t.Fatalf("推荐位缺课程应 400: %d", got.Code)
	}
}
