package exam

import (
	"context"
	"testing"
)

func TestRecordAnomalyMarksOnlyNotGrades(t *testing.T) {
	database := examDB(t)
	defer database.Close()
	ctx := context.Background()
	grantEntitlement(t, database, 1, 1)
	paper, _ := BuildPaper(ctx, database, 1, "L1", 0)
	sid, _ := StartSession(ctx, database, 1, 1, "L1", paper)

	// 记录切屏 + 秒答两条异常
	if err := RecordAnomaly(ctx, database, sid, AnomalyBlur, `{"times":2}`); err != nil {
		t.Fatal(err)
	}
	if err := RecordAnomaly(ctx, database, sid, AnomalyTooFast, `{"qno":"L1-01-01","ms":120}`); err != nil {
		t.Fatal(err)
	}

	// 异常只记不判负：成绩照常可判、不受影响
	res, err := SubmitExam(ctx, database, sid, map[string]string{"L1-01-01": "A", "L1-01-02": "对"})
	if err != nil {
		t.Fatal(err)
	}
	if res.AutoScore != 3.0 {
		t.Fatalf("异常不应影响判分: auto_score=%v", res.AutoScore)
	}

	// exam_anomaly 记录存在且 verdict=pending
	var n int
	if err := database.QueryRow(`SELECT COUNT(*) FROM exam_anomaly WHERE exam_id=$1 AND verdict='pending'`, sid).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("应记 2 条 pending 异常，实际 %d", n)
	}
}

func TestRecordAnomalyRejectsInvalidKind(t *testing.T) {
	database := examDB(t)
	defer database.Close()
	if err := RecordAnomaly(context.Background(), database, 1, AnomalyKind("cheat"), ""); err != ErrAnomalyKind {
		t.Fatalf("非法异常类型应报 ErrAnomalyKind，实际 %v", err)
	}
}
