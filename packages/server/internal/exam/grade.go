package exam

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

var (
	ErrSessionNotFound   = errors.New("会话不存在")
	ErrSessionNotOngoing = errors.New("会话状态不允许交卷")
)

// Result 是交卷判分结果。
type Result struct {
	SessionID  int64
	AutoScore  float64
	TotalScore float64
	IsPass     bool
	Band       string
}

// SubmitExam 交卷并客观自动判分：比对答卷与标准答案，主观题留待阅卷（manual_score=0）。
// 得分可逐题回溯（detail），通过线取课程 cert_pass_lines（上游冻结，不写死）。
func SubmitExam(ctx context.Context, db *sql.DB, sessionID int64, answers map[string]string) (Result, error) {
	var studentID, courseID int64
	var level, paper, status string
	if err := db.QueryRowContext(ctx, `SELECT student_id, course_id, level, paper, status FROM exam_session WHERE id = $1`, sessionID).
		Scan(&studentID, &courseID, &level, &paper, &status); err != nil {
		return Result{}, ErrSessionNotFound
	}
	if status != "ongoing" {
		return Result{}, ErrSessionNotOngoing
	}
	var items []PaperItem
	if err := json.Unmarshal([]byte(paper), &items); err != nil {
		return Result{}, err
	}

	// 取该课程该档位的标准答案与可判分标记。
	type qinfo struct {
		answer string
		grad   string
	}
	qmap := map[string]qinfo{}
	rows, err := db.QueryContext(ctx, `SELECT qno, answer, gradability FROM question WHERE course_id = $1 AND level = $2`, courseID, level)
	if err != nil {
		return Result{}, err
	}
	for rows.Next() {
		var qno string
		var qi qinfo
		var answer sql.NullString
		if err := rows.Scan(&qno, &answer, &qi.grad); err != nil {
			rows.Close()
			return Result{}, err
		}
		qi.answer = answer.String
		qmap[qno] = qi
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return Result{}, err
	}

	// 逐题判分。
	var autoScore float64
	detail := map[string]any{}
	for _, it := range items {
		q, ok := qmap[it.Qno]
		if !ok {
			continue
		}
		switch q.grad {
		case "auto":
			if answers[it.Qno] == q.answer && q.answer != "" {
				autoScore += it.Score
				detail[it.Qno] = map[string]any{"correct": true, "score": it.Score}
			} else {
				detail[it.Qno] = map[string]any{"correct": false, "score": 0}
			}
		default: // manual / semi_auto：留待阅卷
			detail[it.Qno] = map[string]any{"manual": true, "score": 0}
		}
	}

	passLine := passLineFor(ctx, db, courseID, level)
	isPass := autoScore >= passLine
	band := "fail"
	if isPass {
		band = "pass"
	}

	now := time.Now().UTC().Format(time.RFC3339)
	ansJSON, _ := json.Marshal(answers)
	detailJSON, _ := json.Marshal(detail)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Result{}, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx,
		`UPDATE exam_session SET answers = $1, status = 'graded', submitted_at = $2 WHERE id = $3`,
		string(ansJSON), now, sessionID); err != nil {
		return Result{}, err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO exam_result(exam_session_id, student_id, course_id, level, auto_score, manual_score, total_score, band, is_pass, graded_at, detail)
		 VALUES($1,$2,$3,$4,$5,0,$6,$7,$8,$9,$10)`,
		sessionID, studentID, courseID, level, autoScore, autoScore, band, isPass, now, string(detailJSON)); err != nil {
		return Result{}, err
	}
	if err := tx.Commit(); err != nil {
		return Result{}, err
	}
	return Result{SessionID: sessionID, AutoScore: autoScore, TotalScore: autoScore, IsPass: isPass, Band: band}, nil
}

// rowQuerier 抽象 *sql.DB 与 *sql.Tx 共有的单行查询能力。
type rowQuerier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// passLineFor 读取课程的档位合格线（cert_pass_lines 来自上游冻结，不写死）。
func passLineFor(ctx context.Context, q rowQuerier, courseID int64, level string) float64 {
	var raw sql.NullString
	_ = q.QueryRowContext(ctx, `SELECT cert_pass_lines FROM course WHERE id = $1`, courseID).Scan(&raw)
	if !raw.Valid {
		return 0
	}
	var lines map[string]float64
	if err := json.Unmarshal([]byte(raw.String), &lines); err != nil {
		return 0
	}
	return lines[level]
}
