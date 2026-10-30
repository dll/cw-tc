package notify

import (
	"context"
	"testing"
)

func TestUnreadMarkAllAndPrefs(t *testing.T) {
	database := notifyDB(t)
	defer database.Close()
	ctx := context.Background()
	if _, err := database.Exec(`INSERT INTO notification(student_id,type,title,body) VALUES(1,'system','系统','正文'),(1,'order','支付','正文'),(2,'system','他人','正文')`); err != nil {
		t.Fatal(err)
	}
	if n, err := UnreadCount(ctx, database, 1); err != nil || n != 2 {
		t.Fatalf("未读数异常: %d %v", n, err)
	}
	if n, err := MarkAllRead(ctx, database, 1); err != nil || n != 2 {
		t.Fatalf("全部已读异常: %d %v", n, err)
	}
	if n, _ := UnreadCount(ctx, database, 1); n != 0 {
		t.Fatalf("全部已读后未读应为 0: %d", n)
	}
	if err := SetPrefs(ctx, database, 1, NotificationPrefs{BroadcastOptOut: true}); err != nil {
		t.Fatal(err)
	}
	prefs, err := GetPrefs(ctx, database, 1)
	if err != nil || !prefs.BroadcastOptOut {
		t.Fatalf("偏好保存失败: %+v %v", prefs, err)
	}
	if prefs, err := GetPrefs(ctx, database, 2); err != nil || prefs.BroadcastOptOut {
		t.Fatalf("未设置偏好应为默认值: %+v %v", prefs, err)
	}
}

func TestBroadcastSkipsOptedOutStudent(t *testing.T) {
	database := notifyDB(t)
	defer database.Close()
	ctx := context.Background()
	if err := SetPrefs(ctx, database, 1, NotificationPrefs{BroadcastOptOut: true}); err != nil {
		t.Fatal(err)
	}
	id, err := CreateBroadcast(ctx, database, BroadcastInput{
		OperatorID: 1, Target: "all", Title: "新课", Body: "上架", IdempotencyKey: "opt-out",
	})
	if err != nil {
		t.Fatal(err)
	}
	if n, err := ConfirmBroadcast(ctx, database, id, 1); err != nil || n != 1 {
		t.Fatalf("退订学员应被跳过: n=%d err=%v", n, err)
	}
	if n, _ := UnreadCount(ctx, database, 1); n != 0 {
		t.Fatalf("退订学员不应收到广播: %d", n)
	}
	if n, _ := UnreadCount(ctx, database, 2); n != 1 {
		t.Fatalf("其他学员应收到广播: %d", n)
	}
}

func TestNotificationHelpersRejectInvalidInput(t *testing.T) {
	database := notifyDB(t)
	defer database.Close()
	ctx := context.Background()
	if _, err := UnreadCount(ctx, nil, 1); err == nil {
		t.Fatal("nil db 应报错")
	}
	if _, err := UnreadCount(ctx, database, 0); err == nil {
		t.Fatal("非法学员应报错")
	}
	if _, err := MarkAllRead(ctx, database, 0); err == nil {
		t.Fatal("非法学员应报错")
	}
	if _, err := GetPrefs(ctx, database, 0); err == nil {
		t.Fatal("非法学员应报错")
	}
	if err := SetPrefs(ctx, database, 0, NotificationPrefs{}); err == nil {
		t.Fatal("非法学员应报错")
	}
}
