package stats_test

import (
	"context"
	"database/sql"
	"reflect"
	"testing"
	"time"

	"github.com/cw-tc/server/internal/platform/db"
	"github.com/cw-tc/server/internal/stats"
	"github.com/cw-tc/server/migrations"
)

func setup(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.Open(context.Background(), db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.ApplyMigrations(context.Background(), database, migrations.FS, db.DialectSQLite); err != nil {
		database.Close()
		t.Fatal(err)
	}
	stmts := []string{
		`INSERT INTO student(id, student_no, name, phone, id_card_hash, id_card_cipher, agreement_versions) VALUES(1,'S00000001','样例学员','13812345678',X'01',X'02','[]')`,
		`INSERT INTO student(id, student_no, name, phone, id_card_hash, id_card_cipher, agreement_versions) VALUES(2,'S00000002','样例学员二','13912345678',X'03',X'02','[]')`,
		`INSERT INTO staff(id, account, name, role, password_hash, created_source) VALUES(1,'t','样例教师','teacher',X'01','seed')`,
		`INSERT INTO major(id, name) VALUES(1,'样例专业')`,
		`INSERT INTO course(id, course_code, source, name, line, major_id, status, teacher_id, price_cents) VALUES(1,'C1','platform','样例课程','main',1,'published',1,1000)`,
		`INSERT INTO lecture(id, course_id, no, title) VALUES(1,1,1,'第一讲')`,
	}
	for _, stmt := range stmts {
		if _, err := database.Exec(stmt); err != nil {
			database.Close()
			t.Fatal(err)
		}
	}
	at := "2026-10-30T02:00:00Z"
	more := []string{
		`INSERT INTO favorite(student_id, course_code, created_at) VALUES(1,'C1','` + at + `')`,
		`INSERT INTO course_view(course_code, student_id, viewed_at) VALUES('C1',1,'` + at + `')`,
		`INSERT INTO course_view(course_code, student_id, viewed_at) VALUES('C1',NULL,'` + at + `')`,
		`INSERT INTO course_view(course_code, student_id, viewed_at) VALUES('C1',NULL,'` + at + `')`,
		`INSERT INTO entitlement(student_id, course_id, source, granted_at) VALUES(1,1,'free','2026-10-01T00:00:00Z')`,
		`INSERT INTO lecture_progress(student_id, lecture_id, last_studied_at, completed_at) VALUES(1,1,'` + at + `','` + at + `')`,
		`INSERT INTO exam_session(student_id, course_id, level, paper, status, server_deadline_at, created_at) VALUES(1,1,'L1','{}','submitted','2026-10-30T03:00:00Z','` + at + `')`,
		`INSERT INTO certificate(cert_no, student_id, course_id, level, score, band, issued_at, expires_at) VALUES('CW-CN-A202600001',1,1,'L1',90,'excellent','` + at + `','2029-10-30T02:00:00Z')`,
		`INSERT INTO orders(order_no, student_id, course_id, course_name_snapshot, price_snapshot_cents, amount_cents, status, expired_at, paid_at, created_at) VALUES('O1',1,1,'样例课程',1000,1000,'paid','2026-10-30T03:00:00Z','` + at + `','` + at + `')`,
	}
	for _, stmt := range more {
		if _, err := database.Exec(stmt); err != nil {
			database.Close()
			t.Fatal(err)
		}
	}
	return database
}

func TestRecomputeIsIdempotentAndUsesDedupCounts(t *testing.T) {
	database := setup(t)
	defer database.Close()
	ctx := context.Background()
	statDate := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	now := time.Date(2026, 10, 31, 1, 0, 0, 0, time.UTC)

	first, err := stats.Recompute(ctx, database, "C1", statDate, now)
	if err != nil {
		t.Fatal(err)
	}
	second, err := stats.Recompute(ctx, database, "C1", statDate, now)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("同一天重复跑批结果必须一致:\n%+v\n%+v", first, second)
	}
	if first.ViewsUV != 3 || first.Favorites != 1 {
		t.Fatalf("浏览 / 收藏口径异常: %+v", first)
	}
	if first.Learning != 1 || first.Completed != 1 {
		t.Fatalf("在学 / 学完口径异常: %+v", first)
	}
	if first.Examined != 1 || first.Certified != 1 || first.Orders != 1 || first.Paid != 1 || first.Unlock != 1 {
		t.Fatalf("漏斗口径异常: %+v", first)
	}
	if first.StatDate != "2026-10-30" {
		t.Fatalf("统计日期应按 Asia/Shanghai 切日: %s", first.StatDate)
	}

	latest, err := stats.Latest(ctx, database, "C1")
	if err != nil || latest == nil || latest.ViewsUV != 3 {
		t.Fatalf("最新快照异常: %+v %v", latest, err)
	}
	series, err := stats.Range(ctx, database, "C1", "2026-10-30", "2026-10-30")
	if err != nil || len(series) != 1 {
		t.Fatalf("区间查询异常: %d %v", len(series), err)
	}
}

func TestRecomputeAllSkipsDeletedCourses(t *testing.T) {
	database := setup(t)
	defer database.Close()
	ctx := context.Background()
	if _, err := database.Exec(`INSERT INTO course(id, course_code, source, name, line, major_id, status, teacher_id) VALUES(2,'C2','platform','已删除','main',1,'published',1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`UPDATE course SET deleted_at='2026-10-01T00:00:00Z' WHERE id=2`); err != nil {
		t.Fatal(err)
	}
	items, err := stats.RecomputeAll(ctx, database, time.Now().UTC(), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].CourseCode != "C1" {
		t.Fatalf("跑批应跳过已删除课程: %+v", items)
	}
}

func TestDateHelpersAndMissingSnapshot(t *testing.T) {
	if got := stats.DateOf(time.Date(2026, 10, 30, 17, 0, 0, 0, time.UTC)); got != "2026-10-31" {
		t.Fatalf("UTC 17:00 应换算为上海次日: %s", got)
	}
	if _, ok := stats.ParseDate("2026-10-30"); !ok {
		t.Fatal("合法日期应解析成功")
	}
	if _, ok := stats.ParseDate("2026/10/30"); ok {
		t.Fatal("非法日期应拒绝")
	}
	database := setup(t)
	defer database.Close()
	if latest, err := stats.Latest(context.Background(), database, "NOPE"); err != nil || latest != nil {
		t.Fatalf("未跑批应返回 nil 而不是编造数字: %+v %v", latest, err)
	}
	if _, err := stats.Range(context.Background(), database, "C1", "", "2026-10-30"); err == nil {
		t.Fatal("缺少区间应报错")
	}
}

func TestRecomputeErrorsAndEmptyCourse(t *testing.T) {
	database := setup(t)
	defer database.Close()
	ctx := context.Background()
	if _, err := stats.Recompute(ctx, nil, "C1", time.Now(), time.Now()); err == nil {
		t.Fatal("空库应报错")
	}
	if _, err := stats.Recompute(ctx, database, "NOPE", time.Now(), time.Now()); err == nil {
		t.Fatal("不存在课程应报错")
	}
	if _, err := stats.Recompute(ctx, database, "", time.Now(), time.Now()); err == nil {
		t.Fatal("空课程码应报错")
	}
	// 零值时间走默认分支，且无讲次时 completed 保持 0。
	if _, err := database.Exec(`INSERT INTO course(id, course_code, source, name, line, major_id, status, teacher_id) VALUES(2,'C2','platform','无讲次课程','main',1,'published',1)`); err != nil {
		t.Fatal(err)
	}
	empty, err := stats.Recompute(ctx, database, "C2", time.Time{}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if empty.Completed != 0 || empty.Learning != 0 {
		t.Fatalf("无讲次课程不应产生完成人数: %+v", empty)
	}
	if _, err := stats.Latest(ctx, nil, "C1"); err == nil {
		t.Fatal("Latest 空库应报错")
	}
	if _, err := stats.Latest(ctx, database, ""); err == nil {
		t.Fatal("Latest 空课程码应报错")
	}
	if _, err := stats.Range(ctx, nil, "C1", "2026-10-01", "2026-10-30"); err == nil {
		t.Fatal("Range 空库应报错")
	}
}

func TestRecomputeAllEmptyDatabase(t *testing.T) {
	database, err := db.Open(context.Background(), db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := db.ApplyMigrations(context.Background(), database, migrations.FS, db.DialectSQLite); err != nil {
		t.Fatal(err)
	}
	items, err := stats.RecomputeAll(context.Background(), database, time.Now(), time.Now())
	if err != nil || len(items) != 0 {
		t.Fatalf("空库跑批应返回 0 条: %d %v", len(items), err)
	}
	if _, err := stats.RecomputeAll(context.Background(), nil, time.Now(), time.Now()); err == nil {
		t.Fatal("RecomputeAll 空库应报错")
	}
}
