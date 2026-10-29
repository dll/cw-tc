package course

import (
	"context"
	"testing"
	"time"
)

func TestOperationSlotLifecycle(t *testing.T) {
	database := courseDB(t)
	defer database.Close()
	ctx := context.Background()
	if _, err := database.Exec(`INSERT INTO course(course_code,source,name,line,major_id,status,teacher_id) VALUES
		('C1','platform','已上架','main',1,'published',1),
		('C2','platform','建设中','main',1,'building',1)`); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	slot := OperationSlot{
		Position: "home_banner", CourseCode: "C1", Title: "新学期推荐",
		StartsAt: now.Add(-time.Hour).Format(time.RFC3339),
		EndsAt:   now.Add(time.Hour).Format(time.RFC3339),
		Weight:   10, Status: "active",
	}
	id, err := CreateOperationSlot(ctx, database, slot, 1)
	if err != nil {
		t.Fatal(err)
	}
	items, err := PublicOperationSlots(ctx, database, "home_banner", now)
	if err != nil || len(items) != 1 {
		t.Fatalf("公开运营位=%d err=%v", len(items), err)
	}
	// 更新运营位
	slot.ID = id
	slot.Title = "更新后的推荐"
	slot.Weight = 20
	if err := UpdateOperationSlot(ctx, database, id, slot, 1); err != nil {
		t.Fatalf("更新运营位失败: %v", err)
	}
	adminItems, err := ListOperationSlots(ctx, database, now)
	if err != nil || len(adminItems) != 1 || adminItems[0].Title != "更新后的推荐" {
		t.Fatalf("更新后列表异常: %#v %v", adminItems, err)
	}
	// 未上架课程不能关联
	bad := slot
	bad.CourseCode = "C2"
	if _, err := CreateOperationSlot(ctx, database, bad, 1); err != ErrSlotInput {
		t.Fatalf("未上架课程应拒绝: %v", err)
	}
	// 推荐位必须关联课程
	bad = slot
	bad.Position = "home_recommend"
	bad.CourseCode = ""
	if _, err := CreateOperationSlot(ctx, database, bad, 1); err != ErrSlotInput {
		t.Fatalf("推荐位缺课程应拒绝: %v", err)
	}
	// 课程下架自动移除
	if err := UnpublishCourse(ctx, database, "C1", 1); err != nil {
		t.Fatal(err)
	}
	var status string
	if err := database.QueryRow(`SELECT status FROM operation_slot WHERE id=$1`, id).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "removed" {
		t.Fatalf("课程下架后运营位应 removed，实际 %s", status)
	}
	items, _ = PublicOperationSlots(ctx, database, "home_banner", now)
	if len(items) != 0 {
		t.Fatalf("下架后公开运营位应为空，实际 %d", len(items))
	}
}

func TestOperationSlotExpireAndRemove(t *testing.T) {
	database := courseDB(t)
	defer database.Close()
	ctx := context.Background()
	if _, err := database.Exec(`INSERT INTO course(course_code,source,name,line,major_id,status,teacher_id) VALUES('C1','platform','已上架','main',1,'published',1)`); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	expired := OperationSlot{
		Position: "plaza_top", CourseCode: "C1", Title: "已过期",
		StartsAt: now.Add(-2 * time.Hour).Format(time.RFC3339),
		EndsAt:   now.Add(-time.Hour).Format(time.RFC3339),
		Status:   "active",
	}
	id, err := CreateOperationSlot(ctx, database, expired, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := ExpireDueSlots(ctx, database, now); err != nil {
		t.Fatal(err)
	}
	var status string
	if err := database.QueryRow(`SELECT status FROM operation_slot WHERE id=$1`, id).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "expired" {
		t.Fatalf("到期运营位应 expired，实际 %s", status)
	}
	if err := RemoveOperationSlot(ctx, database, id); err != nil {
		t.Fatal(err)
	}
	_ = database.QueryRow(`SELECT status FROM operation_slot WHERE id=$1`, id).Scan(&status)
	if status != "removed" {
		t.Fatalf("下线后应 removed，实际 %s", status)
	}
	if err := RemoveOperationSlot(ctx, database, 999); err != ErrSlotNotFound {
		t.Fatalf("不存在运营位应 ErrSlotNotFound: %v", err)
	}
}
