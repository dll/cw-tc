package exam

import (
	"context"
	"testing"
	"time"
)

func TestSaveAnswerAndResume(t *testing.T) {
	database := examDB(t)
	defer database.Close()
	ctx := context.Background()
	grantEntitlement(t, database, 1, 1)
	paper, _ := BuildPaper(ctx, database, 1, "L1", 0)
	sid, err := StartSession(ctx, database, 1, 1, "L1", paper)
	if err != nil {
		t.Fatal(err)
	}

	// 按题持久化两题
	if err := SaveAnswer(ctx, database, sid, "L1-01-01", "A"); err != nil {
		t.Fatal(err)
	}
	if err := SaveAnswer(ctx, database, sid, "L1-01-02", "对"); err != nil {
		t.Fatal(err)
	}

	// 重连恢复：已答内容应完整
	state, err := Resume(ctx, database, sid, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if state.Answers["L1-01-01"] != "A" || state.Answers["L1-01-02"] != "对" {
		t.Fatalf("恢复答案不完整: %+v", state.Answers)
	}
}

func TestResumeUsesServerClockNotClient(t *testing.T) {
	database := examDB(t)
	defer database.Close()
	ctx := context.Background()
	grantEntitlement(t, database, 1, 1)
	paper, _ := BuildPaper(ctx, database, 1, "L1", 0)
	sid, _ := StartSession(ctx, database, 1, 1, "L1", paper)

	// 把截止时间钉在一个已知值：now + 100s
	now := time.Now().UTC()
	deadline := now.Add(100 * time.Second)
	if _, err := database.Exec(`UPDATE exam_session SET server_deadline_at = $1 WHERE id = $2`, deadline.Format(time.RFC3339), sid); err != nil {
		t.Fatal(err)
	}

	// 客户端谎报时间（把 now 往前调 1 小时）不影响剩余时间——剩余时间由服务端 now 计算
	state, err := Resume(ctx, database, sid, now)
	if err != nil {
		t.Fatal(err)
	}
	if state.RemainingSecs < 99 || state.RemainingSecs > 101 {
		t.Fatalf("剩余时间应由服务端计算（约 100s），实际 %d", state.RemainingSecs)
	}
	if state.Expired {
		t.Fatal("未到期不应标记 expired")
	}

	// 服务端时间已过截止 → expired
	late, err := Resume(ctx, database, sid, now.Add(101*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if !late.Expired || late.RemainingSecs > 0 {
		t.Fatalf("已超时应 expired: expired=%v remaining=%d", late.Expired, late.RemainingSecs)
	}
}

func TestSaveAnswerRejectsNonOngoing(t *testing.T) {
	database := examDB(t)
	defer database.Close()
	ctx := context.Background()
	grantEntitlement(t, database, 1, 1)
	paper, _ := BuildPaper(ctx, database, 1, "L1", 0)
	sid, _ := StartSession(ctx, database, 1, 1, "L1", paper)
	if _, err := SubmitExam(ctx, database, sid, map[string]string{}); err != nil {
		t.Fatal(err)
	}
	if err := SaveAnswer(ctx, database, sid, "L1-01-01", "A"); err != ErrSessionNotOngoing {
		t.Fatalf("已交卷保存答案应报 ErrSessionNotOngoing，实际 %v", err)
	}
}
