package notify

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/cw-tc/server/internal/platform/db"
	"github.com/cw-tc/server/migrations"
)

func notifyDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.Open(context.Background(), db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.ApplyMigrations(context.Background(), database, migrations.FS, db.DialectSQLite); err != nil {
		t.Fatal(err)
	}
	// 两个学员 + 一个超管
	if _, err := database.Exec(`INSERT INTO student(id, student_no, name, phone, id_card_hash, id_card_cipher, agreement_versions) VALUES(1,'S0001','学员一','13800000001',X'01',X'02','v1'),(2,'S0002','学员二','13800000002',X'03',X'04','v1')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO staff(id, account, name, role, password_hash, created_source) VALUES(1,'root','超管','admin_super',X'01','seed')`); err != nil {
		t.Fatal(err)
	}
	return database
}

func TestBroadcastLifecycle(t *testing.T) {
	database := notifyDB(t)
	defer database.Close()
	ctx := context.Background()

	// 创建草稿（不发送）
	id, err := CreateBroadcast(ctx, database, BroadcastInput{
		OperatorID: 1, Target: "all", Title: "公告", Body: "正文", IdempotencyKey: "k1",
	})
	if err != nil {
		t.Fatal(err)
	}
	// 同键幂等 → 返回同一草稿
	id2, _ := CreateBroadcast(ctx, database, BroadcastInput{OperatorID: 1, Target: "all", Title: "x", Body: "y", IdempotencyKey: "k1"})
	if id2 != id {
		t.Fatalf("幂等键应返回同一草稿: %d != %d", id2, id)
	}
	// 未确认前无人收到
	items, _ := ListNotifications(ctx, database, 1, false, 100, 0)
	if len(items) != 0 {
		t.Fatalf("草稿不应扩散通知，实际 %d", len(items))
	}
	// 非创建者确认 → 拒绝
	if _, err := ConfirmBroadcast(ctx, database, id, 999); err != ErrBroadcastForbidden {
		t.Fatalf("非创建者应报 ErrBroadcastForbidden，实际 %v", err)
	}
	// 二次确认发送 → 2 个学员都收到
	n, err := ConfirmBroadcast(ctx, database, id, 1)
	if err != nil || n != 2 {
		t.Fatalf("发送失败或人数不对: n=%d err=%v", n, err)
	}
	// 幂等重发 → 不重复扩散
	n2, err := ConfirmBroadcast(ctx, database, id, 1)
	if err != nil || n2 != 2 {
		t.Fatalf("幂等重发失败: n=%d err=%v", n2, err)
	}
	var cnt int
	_ = database.QueryRow(`SELECT COUNT(*) FROM notification WHERE student_id=1`).Scan(&cnt)
	if cnt != 1 {
		t.Fatalf("重发不应重复扩散，学员1应只有1条，实际 %d", cnt)
	}
	// 标记已读
	list, _ := ListNotifications(ctx, database, 1, true, 100, 0)
	if len(list) != 1 {
		t.Fatalf("未读应 1 条，实际 %d", len(list))
	}
	if err := MarkRead(ctx, database, list[0].ID, 1); err != nil {
		t.Fatal(err)
	}
	unread, _ := ListNotifications(ctx, database, 1, true, 100, 0)
	if len(unread) != 0 {
		t.Fatalf("标记已读后未读应为 0，实际 %d", len(unread))
	}
}

func TestBroadcastValidationAndTargets(t *testing.T) {
	database := notifyDB(t)
	defer database.Close()
	ctx := context.Background()
	if _, err := database.Exec(`INSERT INTO major(id,name) VALUES(1,'专业一')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO course(id, course_code, source, name, line, major_id, status, teacher_id) VALUES(1,'C1','platform','课程一','main',1,'published',1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO entitlement(student_id, course_id, source, granted_at) VALUES(1,1,'free','2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	// 学员 2 退订广播，不应接收。
	if _, err := database.Exec(`INSERT INTO notification_pref(student_id, broadcast_opt_out) VALUES(2,1)`); err != nil {
		t.Fatal(err)
	}
	// 学员 2 也购课，但已退订。
	if _, err := database.Exec(`INSERT INTO entitlement(student_id, course_id, source, granted_at) VALUES(2,1,'free','2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}

	for _, in := range []BroadcastInput{
		{},
		{OperatorID: 1, Target: "bad", Title: "t", Body: "b", IdempotencyKey: "k"},
		{OperatorID: 1, Target: "all", Title: "", Body: "b", IdempotencyKey: "k"},
		{OperatorID: 1, Target: "all", Title: "t", Body: "", IdempotencyKey: "k"},
		{OperatorID: 1, Target: "all", Title: "t", Body: "b", IdempotencyKey: ""},
	} {
		if _, err := CreateBroadcast(ctx, database, in); !errors.Is(err, ErrBroadcastInput) {
			t.Fatalf("非法广播应拒绝: %+v -> %v", in, err)
		}
	}
	if _, err := CreateBroadcast(ctx, nil, BroadcastInput{OperatorID: 1, Target: "all", Title: "t", Body: "b", IdempotencyKey: "k"}); !errors.Is(err, ErrBroadcastInput) {
		t.Fatalf("空库应拒绝: %v", err)
	}
	if _, err := ConfirmBroadcast(ctx, database, 999, 1); !errors.Is(err, ErrBroadcastNotFound) {
		t.Fatalf("不存在广播应 404: %v", err)
	}
	if _, err := ConfirmBroadcast(ctx, database, 0, 1); !errors.Is(err, ErrBroadcastInput) {
		t.Fatalf("非法广播号应拒绝: %v", err)
	}

	courseID, err := CreateBroadcast(ctx, database, BroadcastInput{
		OperatorID: 1, Target: "course", TargetValue: "C1", Title: "课程广播", Body: "正文", IdempotencyKey: "kc",
	})
	if err != nil {
		t.Fatal(err)
	}
	n, err := ConfirmBroadcast(ctx, database, courseID, 1)
	if err != nil || n != 1 {
		t.Fatalf("按课程广播应只触达未退订学员: n=%d err=%v", n, err)
	}
	majorID, err := CreateBroadcast(ctx, database, BroadcastInput{
		OperatorID: 1, Target: "major", TargetValue: "1", Title: "专业广播", Body: "正文", IdempotencyKey: "km",
	})
	if err != nil {
		t.Fatal(err)
	}
	if n, err := ConfirmBroadcast(ctx, database, majorID, 1); err != nil || n != 1 {
		t.Fatalf("按专业广播应只触达未退订学员: n=%d err=%v", n, err)
	}
}

func TestMarkReadOwnershipAndMissing(t *testing.T) {
	database := notifyDB(t)
	defer database.Close()
	ctx := context.Background()
	if _, err := database.Exec(`INSERT INTO notification(student_id, type, title, body) VALUES(1,'system','标题','正文')`); err != nil {
		t.Fatal(err)
	}
	if err := MarkRead(ctx, database, 1, 2); !errors.Is(err, ErrNotificationNotFound) {
		t.Fatalf("他人通知不可标记: %v", err)
	}
	if err := MarkRead(ctx, database, 999, 1); !errors.Is(err, ErrNotificationNotFound) {
		t.Fatalf("不存在通知应 404: %v", err)
	}
	if _, err := ListNotifications(ctx, database, 1, false, 0, 0); err != nil {
		t.Fatalf("limit 越界应回退默认值: %v", err)
	}
}
