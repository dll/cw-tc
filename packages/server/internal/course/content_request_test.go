package course

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

func contentRequestDB(t *testing.T) *sql.DB {
	t.Helper()
	database := courseDB(t)
	if _, err := database.Exec(`INSERT INTO course(id, course_code, source, name, line, major_id, status, teacher_id) VALUES(10,'CR-1','platform','待维护课程','main',1,'published',1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO course(id, course_code, source, name, line, major_id, status, teacher_id) VALUES(11,'CR-2','platform','草稿课程','main',1,'building',1)`); err != nil {
		t.Fatal(err)
	}
	return database
}

func TestCreateContentRequestValidation(t *testing.T) {
	database := contentRequestDB(t)
	defer database.Close()
	ctx := context.Background()

	if _, err := CreateContentRequest(ctx, nil, ContentRequest{}); !errors.Is(err, ErrContentRequestInput) {
		t.Fatalf("空库应拒绝: %v", err)
	}
	if _, err := CreateContentRequest(ctx, database, ContentRequest{CourseCode: "CR-1", TeacherID: 1, Kind: "bad", ReasonKind: "other"}); !errors.Is(err, ErrContentRequestInput) {
		t.Fatalf("非法 kind 应拒绝: %v", err)
	}
	if _, err := CreateContentRequest(ctx, database, ContentRequest{CourseCode: "CR-1", TeacherID: 1, Kind: "offline", ReasonKind: "bad"}); !errors.Is(err, ErrContentRequestInput) {
		t.Fatalf("非法理由应拒绝: %v", err)
	}
	if _, err := CreateContentRequest(ctx, database, ContentRequest{CourseCode: "NOPE", TeacherID: 1, Kind: "offline", ReasonKind: "other"}); !errors.Is(err, ErrCourseNotFound) {
		t.Fatalf("不存在课程应拒绝: %v", err)
	}
	// 非本课教师（归属不匹配）同样视为课程不存在。
	if _, err := CreateContentRequest(ctx, database, ContentRequest{CourseCode: "CR-1", TeacherID: 2, Kind: "offline", ReasonKind: "other"}); !errors.Is(err, ErrCourseNotFound) {
		t.Fatalf("非归属教师应拒绝: %v", err)
	}
	// 未上架课程不能申请下架。
	if _, err := CreateContentRequest(ctx, database, ContentRequest{CourseCode: "CR-2", TeacherID: 1, Kind: "offline", ReasonKind: "other"}); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("未上架课程应拒绝: %v", err)
	}
}

func TestCreateListAndHandleContentRequest(t *testing.T) {
	database := contentRequestDB(t)
	defer database.Close()
	ctx := context.Background()

	id, err := CreateContentRequest(ctx, database, ContentRequest{
		CourseCode: "CR-1", TeacherID: 1, Kind: "offline", ReasonKind: "content_error",
		ReasonText: "例题答案写错了，需要下架核对。",
	})
	if err != nil || id <= 0 {
		t.Fatalf("创建申请失败: %d %v", id, err)
	}
	// 同课程同类型只允许一条 pending。
	if _, err := CreateContentRequest(ctx, database, ContentRequest{
		CourseCode: "CR-1", TeacherID: 1, Kind: "offline", ReasonKind: "other",
	}); !errors.Is(err, ErrContentRequestExists) {
		t.Fatalf("重复 pending 应拒绝: %v", err)
	}
	// 不同类型可并存（urgent_fix）。
	if _, err := CreateContentRequest(ctx, database, ContentRequest{
		CourseCode: "CR-1", TeacherID: 1, Kind: "urgent_fix", ReasonKind: "material_change",
	}); err != nil {
		t.Fatalf("不同 kind 应可创建: %v", err)
	}
	if !HasPendingUrgentFix(ctx, database, "CR-1") {
		t.Fatal("应识别出待处理的紧急修课申请")
	}
	if HasPendingUrgentFix(ctx, database, "NOPE") || HasPendingUrgentFix(ctx, nil, "CR-1") {
		t.Fatal("无申请 / 空库应返回 false")
	}

	items, err := ListContentRequests(ctx, database, "pending", 1, 0, 0)
	if err != nil || len(items) != 2 {
		t.Fatalf("教师待办应为 2 条: %d %v", len(items), err)
	}
	if items[0].CourseName != "待维护课程" || items[0].TeacherName == "" {
		t.Fatalf("列表应带课程与教师名称: %+v", items[0])
	}
	if _, err := ListContentRequests(ctx, nil, "", 0, 20, 0); !errors.Is(err, ErrContentRequestInput) {
		t.Fatalf("空库列表应报错: %v", err)
	}
	if items, err := ListContentRequests(ctx, database, "", 0, 200, 0); err != nil || len(items) != 2 {
		t.Fatalf("limit 超上限应回退默认值: %d %v", len(items), err)
	}

	// 参数非法 / 不存在 / 重复处理。
	if _, err := HandleContentRequest(ctx, nil, 1, 1, true); !errors.Is(err, ErrContentRequestInput) {
		t.Fatalf("空库处理应拒绝: %v", err)
	}
	if _, err := HandleContentRequest(ctx, database, 999, 1, true); !errors.Is(err, ErrContentRequestNotFound) {
		t.Fatalf("不存在申请应拒绝: %v", err)
	}
	handled, err := HandleContentRequest(ctx, database, id, 1, true)
	if err != nil || handled.Status != "approved" || handled.HandledBy == nil {
		t.Fatalf("批准申请失败: %+v %v", handled, err)
	}
	var status string
	if err := database.QueryRow(`SELECT status FROM course WHERE course_code='CR-1'`).Scan(&status); err != nil || status != "unpublished" {
		t.Fatalf("批准后课程应下架: %s %v", status, err)
	}
	if _, err := HandleContentRequest(ctx, database, id, 1, false); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("重复处理应拒绝: %v", err)
	}
	// 驳回路径。
	urgent, _ := ListContentRequests(ctx, database, "pending", 1, 20, 0)
	if len(urgent) != 1 {
		t.Fatalf("应剩 1 条紧急修课申请: %+v", urgent)
	}
	rejected, err := HandleContentRequest(ctx, database, urgent[0].ID, 1, false)
	if err != nil || rejected.Status != "rejected" {
		t.Fatalf("驳回申请失败: %+v %v", rejected, err)
	}
}
