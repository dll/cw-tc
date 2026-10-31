package offline_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/cw-tc/server/internal/offline"
	"github.com/cw-tc/server/internal/platform/db"
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
		`INSERT INTO staff(id, account, name, role, password_hash, created_source) VALUES(1,'t','样例教师','teacher',X'01','seed')`,
		`INSERT INTO major(id, name) VALUES(1,'样例专业')`,
		`INSERT INTO course(id, course_code, source, name, line, major_id, status, teacher_id, price_cents) VALUES(1,'C1','platform','样例课程','main',1,'published',1,1000)`,
		`INSERT INTO lecture(course_id, no, title) VALUES(1,1,'第一讲')`,
	}
	for _, stmt := range stmts {
		if _, err := database.Exec(stmt); err != nil {
			database.Close()
			t.Fatal(err)
		}
	}
	return database
}

func TestRequestRequiresUnlockedCourse(t *testing.T) {
	database := setup(t)
	defer database.Close()
	service := offline.New(database)
	ctx := context.Background()

	if _, err := service.Request(ctx, 1, "C1", 1, 30); !errors.Is(err, offline.ErrForbidden) {
		t.Fatalf("未解锁应拒绝，实际 %v", err)
	}
	if _, err := database.Exec(`INSERT INTO entitlement(student_id, course_id, source, granted_at) VALUES(1,1,'free','2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	grant, err := service.Request(ctx, 1, "C1", 1, 30)
	if err != nil {
		t.Fatal(err)
	}
	if grant.Status != "valid" || grant.RemainingDays != 30 || grant.CourseCode != "C1" {
		t.Fatalf("授权异常: %+v", grant)
	}
	// 重复申请幂等：不产生第二条记录。
	again, err := service.Request(ctx, 1, "C1", 1, 30)
	if err != nil || again.ID != grant.ID {
		t.Fatalf("重复申请应复用同一条记录: %+v %v", again, err)
	}
	items, err := service.List(ctx, 1)
	if err != nil || len(items) != 1 {
		t.Fatalf("清单应为 1 条: %d %v", len(items), err)
	}
}

func TestExpireDeleteAndRefundRevocation(t *testing.T) {
	database := setup(t)
	defer database.Close()
	ctx := context.Background()
	now := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	service := offline.NewWithNow(database, func() time.Time { return now })
	if _, err := database.Exec(`INSERT INTO entitlement(student_id, course_id, source, granted_at) VALUES(1,1,'free','2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Request(ctx, 1, "C1", 1, 30); err != nil {
		t.Fatal(err)
	}
	// 31 天后：清单自动置 expired。
	later := offline.NewWithNow(database, func() time.Time { return now.Add(31 * 24 * time.Hour) })
	items, err := later.List(ctx, 1)
	if err != nil || len(items) != 1 || items[0].Status != "expired" {
		t.Fatalf("过期口径异常: %+v %v", items, err)
	}
	// 删除单讲下载。
	if err := later.Delete(ctx, 1, "C1", 1); err != nil {
		t.Fatal(err)
	}
	if err := later.Delete(ctx, 1, "C1", 1); !errors.Is(err, offline.ErrNotFound) {
		t.Fatalf("重复删除应 404，实际 %v", err)
	}
}

func TestRevokeForCourseMarksGrantRevoked(t *testing.T) {
	database := setup(t)
	defer database.Close()
	ctx := context.Background()
	service := offline.New(database)
	if _, err := database.Exec(`INSERT INTO entitlement(student_id, course_id, source, granted_at) VALUES(1,1,'free','2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Request(ctx, 1, "C1", 1, 30); err != nil {
		t.Fatal(err)
	}
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := offline.RevokeForCourse(ctx, tx, 1, 1, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	items, err := service.List(ctx, 1)
	if err != nil || len(items) != 1 || items[0].Status != "revoked" {
		t.Fatalf("退款后应撤销授权: %+v %v", items, err)
	}
	if items[0].RevokedReason == nil || *items[0].RevokedReason != "order_refunded" {
		t.Fatalf("撤销原因应为 order_refunded: %+v", items[0])
	}
}

func TestInputValidationAndMissingTargets(t *testing.T) {
	database := setup(t)
	defer database.Close()
	ctx := context.Background()
	service := offline.New(database)
	if _, err := service.Request(ctx, 0, "C1", 1, 30); !errors.Is(err, offline.ErrInput) {
		t.Fatalf("非法学员应拒绝: %v", err)
	}
	if _, err := service.Request(ctx, 1, "NOPE", 1, 30); !errors.Is(err, offline.ErrCourseNotFound) {
		t.Fatalf("不存在课程应 404: %v", err)
	}
	if _, err := service.Request(ctx, 1, "C1", 99, 30); !errors.Is(err, offline.ErrLectureNotFound) {
		t.Fatalf("不存在讲次应 404: %v", err)
	}
	if err := service.Delete(ctx, 1, "C1", 1); !errors.Is(err, offline.ErrNotFound) {
		t.Fatalf("删除不存在记录应 404: %v", err)
	}
	if err := service.DeleteByLectureNo(ctx, 1, 1); !errors.Is(err, offline.ErrNotFound) {
		t.Fatalf("按讲次删除不存在记录应 404: %v", err)
	}
	if err := service.DeleteByLectureNo(ctx, 0, 1); !errors.Is(err, offline.ErrInput) {
		t.Fatalf("非法学员删除应拒绝: %v", err)
	}
}

func TestRequestRenewsExpiredGrant(t *testing.T) {
	database := setup(t)
	defer database.Close()
	ctx := context.Background()
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	service := offline.NewWithNow(database, func() time.Time { return now })
	if _, err := database.Exec(`INSERT INTO entitlement(student_id, course_id, source, granted_at) VALUES(1,1,'free','2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	first, err := service.Request(ctx, 1, "C1", 1, 30)
	if err != nil {
		t.Fatal(err)
	}
	later := offline.NewWithNow(database, func() time.Time { return now.Add(40 * 24 * time.Hour) })
	renewed, err := later.Request(ctx, 1, "C1", 1, 30)
	if err != nil {
		t.Fatal(err)
	}
	if renewed.ID != first.ID || renewed.Status != "valid" || renewed.GrantedAt == first.GrantedAt {
		t.Fatalf("过期后再次申请应续期同一条记录: %+v -> %+v", first, renewed)
	}
}

func TestListValidationAndRevokeInput(t *testing.T) {
	database := setup(t)
	defer database.Close()
	ctx := context.Background()
	service := offline.New(database)
	if _, err := service.List(ctx, 0); !errors.Is(err, offline.ErrInput) {
		t.Fatalf("非法学员列表应拒绝: %v", err)
	}
	if _, err := service.Request(ctx, 1, "C1", 1, 0); !errors.Is(err, offline.ErrInput) {
		t.Fatalf("有效期非法应拒绝: %v", err)
	}
	if err := offline.RevokeForCourse(ctx, nil, 1, 1, time.Now()); !errors.Is(err, offline.ErrInput) {
		t.Fatalf("空事务应拒绝: %v", err)
	}
	if err := service.Delete(ctx, 0, "C1", 1); !errors.Is(err, offline.ErrInput) {
		t.Fatalf("非法删除参数应拒绝: %v", err)
	}
	if _, err := service.Request(ctx, 1, "C1", 1, 30); !errors.Is(err, offline.ErrForbidden) {
		t.Fatalf("未解锁应拒绝: %v", err)
	}
}

func TestRemainingDaysHandlesBadTimestamp(t *testing.T) {
	database := setup(t)
	defer database.Close()
	ctx := context.Background()
	service := offline.New(database)
	if _, err := database.Exec(`INSERT INTO entitlement(student_id, course_id, source, granted_at) VALUES(1,1,'free','2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO download_grant(student_id, course_id, lecture_no, granted_at, expires_at, status) VALUES(1,1,1,'aaa','bbb','valid')`); err != nil {
		t.Fatal(err)
	}
	items, err := service.List(ctx, 1)
	if err != nil || len(items) != 1 || items[0].RemainingDays != 0 {
		t.Fatalf("坏时间戳应返回 0 天而不是 panic: %+v %v", items, err)
	}
}

func TestNewWithNilClockUsesRealClock(t *testing.T) {
	database := setup(t)
	defer database.Close()
	if offline.NewWithNow(database, nil) == nil {
		t.Fatal("空时钟应回退真实时钟")
	}
	if offline.New(nil) == nil {
		t.Fatal("空库也应返回服务对象")
	}
}