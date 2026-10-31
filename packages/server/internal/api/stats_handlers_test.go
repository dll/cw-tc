package api

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cw-tc/server/internal/platform/db"
	"github.com/cw-tc/server/internal/staff"
	"github.com/cw-tc/server/internal/stats"
	"github.com/cw-tc/server/internal/sysconfig"
	"github.com/cw-tc/server/migrations"
)

func statsServer(t *testing.T) (*Server, *sql.DB, string, string) {
	t.Helper()
	database, err := db.Open(context.Background(), db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.ApplyMigrations(context.Background(), database, migrations.FS, db.DialectSQLite); err != nil {
		database.Close()
		t.Fatal(err)
	}
	if _, err := staff.Create(context.Background(), database, "boss", "超管", staff.RoleAdminSuper, "Temp@123", 0); err != nil {
		t.Fatal(err)
	}
	teacherID, err := staff.Create(context.Background(), database, "t1", "教师一", staff.RoleTeacher, "Temp@123", 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := staff.Create(context.Background(), database, "t2", "教师二", staff.RoleTeacher, "Temp@123", 0); err != nil {
		t.Fatal(err)
	}
	stmts := []string{
		`INSERT INTO major(id, name) VALUES(1,'样例专业')`,
		`INSERT INTO student(id, student_no, name, phone, id_card_hash, id_card_cipher, agreement_versions) VALUES(1,'S00000001','样例学员','13812345678',X'01',X'02','[]')`,
		`INSERT INTO course(id, course_code, source, name, line, major_id, status, teacher_id, price_cents) VALUES(1,'C1','platform','样例课程','main',1,'published',` + strconv.FormatInt(teacherID, 10) + `,0)`,
		`INSERT INTO lecture(course_id, no, title) VALUES(1,1,'第一讲')`,
	}
	for _, stmt := range stmts {
		if _, err := database.Exec(stmt); err != nil {
			database.Close()
			t.Fatal(fmt.Errorf("准备测试数据失败: %w", err))
		}
	}
	server := NewWithDatabase(testConfig(t, true), database)
	r := server.Router()
	super := performJSON(t, r, http.MethodPost, "/api/v1/admin/login", `{"account":"boss","password":"Temp@123"}`)
	teacher := performJSON(t, r, http.MethodPost, "/api/v1/admin/login", `{"account":"t1","password":"Temp@123"}`)
	return server, database, extractJSONField(t, super.Body.String(), "token"),
		extractJSONField(t, teacher.Body.String(), "token")
}

func TestPublicStatsMaskedAndAdminExact(t *testing.T) {
	server, database, superToken, _ := statsServer(t)
	defer database.Close()
	r := server.Router()
	ctx := context.Background()
	at := "2026-10-30T02:00:00Z"
	for _, stmt := range []string{
		`INSERT INTO course_view(course_code, student_id, viewed_at) VALUES('C1',NULL,'` + at + `')`,
		`INSERT INTO course_view(course_code, student_id, viewed_at) VALUES('C1',NULL,'` + at + `')`,
		`INSERT INTO course_view(course_code, student_id, viewed_at) VALUES('C1',NULL,'` + at + `')`,
		`INSERT INTO favorite(student_id, course_code, created_at) VALUES(1,'C1','` + at + `')`,
	} {
		if _, err := database.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := stats.Recompute(ctx, database, "C1", time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC), time.Now().UTC()); err != nil {
		t.Fatal(err)
	}

	// 默认阈值 10：3 人浏览 < 10 → 对外不得显示确切数字，但必须给"数据截至"。
	public := performJSON(t, r, http.MethodGet, "/api/v1/courses/C1", "")
	if public.Code != http.StatusOK || !containsAll(public.Body.String(), `"data_as_of":"2026-10-30"`, `"views_uv":null`) {
		t.Fatalf("小样本应脱敏且带数据截至日期: %d %s", public.Code, public.Body.String())
	}

	// 管理端看精确数。
	insights := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/insights", "", superToken)
	if insights.Code != http.StatusOK || !containsAll(insights.Body.String(), `"views_uv":3`, `"course_code":"C1"`) {
		t.Fatalf("管理端统计应精确: %d %s", insights.Code, insights.Body.String())
	}
	detail := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/stats/C1", "", superToken)
	if detail.Code != http.StatusOK || !containsAll(detail.Body.String(), `"funnel"`, `"data_as_of":"2026-10-30"`) {
		t.Fatalf("单课统计异常: %d %s", detail.Code, detail.Body.String())
	}
}

func TestStatsThresholdConfigurableTakesEffect(t *testing.T) {
	server, database, _, _ := statsServer(t)
	defer database.Close()
	r := server.Router()
	ctx := context.Background()
	at := "2026-10-30T02:00:00Z"
	for i := 0; i < 3; i++ {
		if _, err := database.Exec(`INSERT INTO course_view(course_code, student_id, viewed_at) VALUES('C1',NULL,'` + at + `')`); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := sysconfig.New(database).Set(ctx, sysconfig.KeyStatsDisplayThreshold, 3, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := stats.Recompute(ctx, database, "C1", time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC), time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	public := performJSON(t, r, http.MethodGet, "/api/v1/courses/C1", "")
	if public.Code != http.StatusOK || !containsAll(public.Body.String(), `"views_uv":3`, `"display_threshold":3`) {
		t.Fatalf("阈值改小后应立即生效: %d %s", public.Code, public.Body.String())
	}
}

func TestTeacherStatsRequiresOwnership(t *testing.T) {
	server, database, _, teacherToken := statsServer(t)
	defer database.Close()
	r := server.Router()
	ok := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/teacher/courses/C1/stats?range=30d", "", teacherToken)
	if ok.Code != http.StatusOK || !containsAll(ok.Body.String(), `"funnel"`, `"range":"30d"`) {
		t.Fatalf("本课统计应可读: %d %s", ok.Code, ok.Body.String())
	}
	other := performJSON(t, r, http.MethodPost, "/api/v1/admin/login", `{"account":"t2","password":"Temp@123"}`)
	otherToken := extractJSONField(t, other.Body.String(), "token")
	if got := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/teacher/courses/C1/stats", "", otherToken); got.Code != http.StatusForbidden {
		t.Fatalf("他人课程统计应 403，实际 %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/teacher/courses/C1/stats?range=1d", "", teacherToken); got.Code != http.StatusBadRequest {
		t.Fatalf("非法 range 应 400，实际 %d", got.Code)
	}
}

func TestAdminSettingsUpdateAndAudit(t *testing.T) {
	server, database, superToken, _ := statsServer(t)
	defer database.Close()
	r := server.Router()
	updated := performJSONWithBearer(t, r, http.MethodPatch, "/api/v1/admin/settings",
		`{"key":"stats.display_threshold","value":5}`, superToken)
	if updated.Code != http.StatusOK || !containsAll(updated.Body.String(), `"value":5`) {
		t.Fatalf("修改配置失败: %d %s", updated.Code, updated.Body.String())
	}
	if got := performJSONWithBearer(t, r, http.MethodPatch, "/api/v1/admin/settings",
		`{"key":"stats.display_threshold","value":1}`, superToken); got.Code != http.StatusBadRequest {
		t.Fatalf("低于下限应 400，实际 %d %s", got.Code, got.Body.String())
	}
	if got := performJSONWithBearer(t, r, http.MethodPatch, "/api/v1/admin/settings",
		`{"key":"secret.token","value":"x"}`, superToken); got.Code != http.StatusNotFound {
		t.Fatalf("未登记键应 404，实际 %d", got.Code)
	}
	list := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/settings", "", superToken)
	if list.Code != http.StatusOK || !containsAll(list.Body.String(), `"stats.display_threshold"`, `"min_value":3`) {
		t.Fatalf("配置列表异常: %d %s", list.Code, list.Body.String())
	}
	var n int
	if err := database.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action='system_config.update'`).Scan(&n); err != nil || n == 0 {
		t.Fatalf("配置修改必须写审计: %d %v", n, err)
	}
}

func TestRecordCourseViewDedupsLoggedInWithin24Hours(t *testing.T) {
	s, token, cleanup := orderLifecycleServer(t)
	defer cleanup()
	r := s.Router()
	for i := 0; i < 2; i++ {
		if got := performJSONWithCookie(t, r, http.MethodGet, "/api/v1/courses/C1", "", token); got.Code != http.StatusOK {
			t.Fatalf("课程详情应可读: %d", got.Code)
		}
	}
	var n int
	if err := s.dbConn.QueryRow(`SELECT COUNT(*) FROM course_view WHERE course_code='C1' AND student_id=1`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("同一登录用户 24h 内浏览应只记 1 次，实际 %d", n)
	}
}

func TestAdminInsightsSourceFilterAndValidation(t *testing.T) {
	server, database, superToken, _ := statsServer(t)
	defer database.Close()
	r := server.Router()
	all := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/insights", "", superToken)
	if all.Code != http.StatusOK || !containsAll(all.Body.String(), `"course_code":"C1"`) {
		t.Fatalf("全量看板异常: %d", all.Code)
	}
	platform := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/insights?source=platform", "", superToken)
	if platform.Code != http.StatusOK || !containsAll(platform.Body.String(), `"source":"platform"`) {
		t.Fatalf("来源筛选异常: %d %s", platform.Code, platform.Body.String())
	}
	upstream := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/insights?source=upstream", "", superToken)
	if upstream.Code != http.StatusOK || strings.Contains(upstream.Body.String(), `"course_code":"C1"`) {
		t.Fatalf("upstream 筛选应排除平台课: %d %s", upstream.Code, upstream.Body.String())
	}
	if got := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/insights?source=bad", "", superToken); got.Code != http.StatusBadRequest {
		t.Fatalf("非法来源应 400，实际 %d", got.Code)
	}
	// 未跑批的单课统计返回 0 与 null 日期，不编造数字。
	detail := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/stats/C1", "", superToken)
	if detail.Code != http.StatusOK || !containsAll(detail.Body.String(), `"data_as_of":null`) {
		t.Fatalf("未跑批应返回 null 日期: %d %s", detail.Code, detail.Body.String())
	}
}
