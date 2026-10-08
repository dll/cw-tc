package exam

import (
	"context"
	"testing"
	"time"
)

func TestRetakeCooldown(t *testing.T) {
	database := examDB(t)
	defer database.Close()
	ctx := context.Background()
	grantEntitlement(t, database, 1, 1)
	paper, _ := BuildPaper(ctx, database, 1, "L1", 0)

	// 第一次开考 + 交卷
	sid1, err := StartSession(ctx, database, 1, 1, "L1", paper)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := SubmitExam(ctx, database, sid1, map[string]string{}); err != nil {
		t.Fatal(err)
	}
	var seq int
	_ = database.QueryRow(`SELECT retake_seq FROM exam_session WHERE id=$1`, sid1).Scan(&seq)
	if seq != 1 {
		t.Fatalf("首次 retake_seq 应为 1，实际 %d", seq)
	}

	// 冷却期内（24h）不可重考
	if _, err := StartSession(ctx, database, 1, 1, "L1", paper); err != ErrRetakeCooldown {
		t.Fatalf("冷却期内应报 ErrRetakeCooldown，实际 %v", err)
	}

	// 把最近交卷时间拨到 25 小时前 → 冷却结束，可重考
	if _, err := database.Exec(`UPDATE exam_session SET submitted_at=$1 WHERE id=$2`,
		time.Now().Add(-25*time.Hour).UTC().Format(time.RFC3339), sid1); err != nil {
		t.Fatal(err)
	}
	sid2, err := StartSession(ctx, database, 1, 1, "L1", paper)
	if err != nil {
		t.Fatalf("冷却结束后应可重考: %v", err)
	}
	_ = database.QueryRow(`SELECT retake_seq FROM exam_session WHERE id=$1`, sid2).Scan(&seq)
	if seq != 2 {
		t.Fatalf("第二次 retake_seq 应为 2，实际 %d", seq)
	}
}
