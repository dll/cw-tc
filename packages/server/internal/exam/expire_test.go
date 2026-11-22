package exam

import (
	"context"
	"testing"
	"time"
)

func TestStartSessionClosesExpiredOngoing(t *testing.T) {
	database := examDB(t)
	defer database.Close()
	ctx := context.Background()
	grantEntitlement(t, database, 1, 1)
	paper, err := BuildPaper(ctx, database, 1, "L1", 0)
	if err != nil {
		t.Fatal(err)
	}
	var oldID int64
	if err := database.QueryRow(`INSERT INTO exam_session(student_id, course_id, level, paper, answers, status, started_at, server_deadline_at)
		VALUES(1,1,'L1',$1,'{"L1-01-01":"A"}','ongoing',$2,$3) RETURNING id`,
		paper,
		time.Now().Add(-2*time.Hour).UTC().Format(time.RFC3339),
		time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)).Scan(&oldID); err != nil {
		t.Fatal(err)
	}
	sid, err := StartSession(ctx, database, 1, 1, "L1", paper)
	if err != nil {
		t.Fatalf("过期会话应自动结束并可重新开考: %v", err)
	}
	if sid <= 0 || sid == oldID {
		t.Fatalf("应生成新会话，old=%d new=%d", oldID, sid)
	}
	var status string
	if err := database.QueryRow(`SELECT status FROM exam_session WHERE id = $1`, oldID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status == "ongoing" {
		t.Fatal("过期会话仍为 ongoing")
	}
}

func TestSubmitExamAfterDeadlineUsesSavedAnswers(t *testing.T) {
	database := examDB(t)
	defer database.Close()
	ctx := context.Background()
	grantEntitlement(t, database, 1, 1)
	paper, _ := BuildPaper(ctx, database, 1, "L1", 0)
	sid, err := StartSession(ctx, database, 1, 1, "L1", paper)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`UPDATE exam_session SET answers = '{"L1-01-01":"A"}', server_deadline_at = $1 WHERE id = $2`,
		time.Now().Add(-time.Minute).UTC().Format(time.RFC3339), sid); err != nil {
		t.Fatal(err)
	}
	// 客户端迟到提交全对答案：服务端应只使用已保存的一题。
	res, err := SubmitExam(ctx, database, sid, map[string]string{
		"L1-01-01": "A", "L1-01-02": "对",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.AutoScore != 1.5 {
		t.Fatalf("迟到提交应按已保存答案判分 1.5，实际 %v", res.AutoScore)
	}
}

func TestSaveAnswerAfterDeadlineRejected(t *testing.T) {
	database := examDB(t)
	defer database.Close()
	ctx := context.Background()
	grantEntitlement(t, database, 1, 1)
	paper, _ := BuildPaper(ctx, database, 1, "L1", 0)
	sid, err := StartSession(ctx, database, 1, 1, "L1", paper)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`UPDATE exam_session SET server_deadline_at = $1 WHERE id = $2`,
		time.Now().Add(-time.Minute).UTC().Format(time.RFC3339), sid); err != nil {
		t.Fatal(err)
	}
	if err := SaveAnswer(ctx, database, sid, "L1-01-01", "A"); err != ErrSessionExpired {
		t.Fatalf("到时后保存答案应拒绝，实际 %v", err)
	}
}

func TestExpireStaleFallbackMarksSubmitted(t *testing.T) {
	database := examDB(t)
	defer database.Close()
	ctx := context.Background()
	var id int64
	if err := database.QueryRow(`INSERT INTO exam_session(student_id, course_id, level, paper, status, started_at, server_deadline_at)
		VALUES(1,1,'L1','{"bad":true}','ongoing',$1,$2) RETURNING id`,
		time.Now().Add(-2*time.Hour).UTC().Format(time.RFC3339),
		time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if _, err := ExpireStale(ctx, database, 1, 1, "L1", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	var status string
	if err := database.QueryRow(`SELECT status FROM exam_session WHERE id = $1`, id).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "submitted" {
		t.Fatalf("判分失败应退化为 submitted，实际 %s", status)
	}
}
