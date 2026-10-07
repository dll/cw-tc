// Package e2e 用 mock 支付跑通「第一单 + 测评」全链路，验证除真实支付外的所有核心功能。
// 真实微信/支付宝商户号审核中，支付走 mock 验签回调（不连外部渠道）。
package e2e

import (
	"bytes"
	"context"
	"database/sql"
	"strconv"
	"testing"
	"time"

	"github.com/cw-tc/server/internal/commerce"
	"github.com/cw-tc/server/internal/exam"
	"github.com/cw-tc/server/internal/identity"
	"github.com/cw-tc/server/internal/platform/db"
	"github.com/cw-tc/server/migrations"
)

const (
	samplePhone = "13812345678"
	sampleID    = "110101199001011237" // 样例号（虚构，校验位合法）
	samplePwd   = "Passw0rd123"
)

func e2eDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.Open(context.Background(), db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.ApplyMigrations(context.Background(), database, migrations.FS, db.DialectSQLite); err != nil {
		t.Fatal(err)
	}
	// 教师 + 专业 + 已上架课程（价格 1000 分，L1 合格线 1.5）
	if _, err := database.Exec(`INSERT INTO staff(id, account, name, role, password_hash, created_source) VALUES(1,'t','教师','teacher',X'01','seed')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO major(id, name) VALUES(1,'样例专业')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO course(id, course_code, source, name, line, major_id, status, teacher_id, price_cents, cert_pass_lines) VALUES(1,'C1','platform','样例课程','main',1,'published',1,1000,'{"L1":1.5}')`); err != nil {
		t.Fatal(err)
	}
	for _, q := range []struct{ qno, typ, grad, stem, ans string }{
		{"L1-01-01", "single", "auto", "赋值是表达式吗？", "A"},
		{"L1-01-02", "judge", "auto", "指针是地址。", "对"},
	} {
		if _, err := database.Exec(`INSERT INTO question(course_id, qno, level, type, gradability, stem, answer, score) VALUES(1,?,'L1',?,?,?,?,1.5)`,
			q.qno, q.typ, q.grad, q.stem, q.ans); err != nil {
			t.Fatal(err)
		}
	}
	return database
}

// TestFullFlowWithMockPayment 全链路：注册→登录→下单→mock支付→权益→组卷→开考→答题→重连→交卷→判分。
func TestFullFlowWithMockPayment(t *testing.T) {
	database := e2eDB(t)
	defer database.Close()
	ctx := context.Background()

	// 1. M1-A 账号：注册 + 登录
	idsvc := identity.NewService(database, bytes.Repeat([]byte("a"), 32), bytes.Repeat([]byte("b"), 32))
	stu, err := idsvc.Register(ctx, identity.RegisterInput{Name: "样例学员", Phone: samplePhone, IDCard: sampleID, Password: samplePwd})
	if err != nil {
		t.Fatalf("注册失败: %v", err)
	}
	if _, err := idsvc.Login(ctx, samplePhone, samplePwd); err != nil {
		t.Fatalf("登录失败: %v", err)
	}

	// 2. M2-G 下单：真实价格快照
	csvc := commerce.NewService(database)
	order, err := csvc.CreateOrderFromCourse(ctx, stu.ID, 1, "e2e-order", time.Now().Add(30*time.Minute))
	if err != nil {
		t.Fatalf("下单失败: %v", err)
	}
	if order.AmountCents != 1000 {
		t.Fatalf("价格快照错误: %d", order.AmountCents)
	}

	// 3. mock 支付回调：验签 + 幂等 → 开通权益
	paysvc := commerce.NewPaymentService(database, []byte("callback-test-secret"))
	cb := commerce.CallbackInput{OrderNo: order.OrderNo, Channel: "mock", ChannelTxnID: "txn-e2e", AmountCents: order.AmountCents, IdempotencyKey: "idem-e2e"}
	cb.Signature = commerce.SignCallback([]byte("callback-test-secret"), order.OrderNo, cb.ChannelTxnID, strconv.FormatInt(order.AmountCents, 10))
	if err := paysvc.HandleCallback(ctx, cb); err != nil {
		t.Fatalf("支付回调失败: %v", err)
	}
	// 幂等：重复回调不重复开通
	if err := paysvc.HandleCallback(ctx, cb); err != nil {
		t.Fatalf("重复回调应幂等: %v", err)
	}
	ents, err := csvc.ListEntitlements(ctx, stu.ID)
	if err != nil || len(ents) != 1 {
		t.Fatalf("权益未正确开通: 条数=%d err=%v", len(ents), err)
	}

	// 4. M3-01 组卷 + 开考（需已解锁权益）
	paper, err := exam.BuildPaper(ctx, database, 1, "L1", 0)
	if err != nil {
		t.Fatalf("组卷失败: %v", err)
	}
	sid, err := exam.StartSession(ctx, database, stu.ID, 1, "L1", paper)
	if err != nil {
		t.Fatalf("开考失败: %v", err)
	}

	// 5. M3-03 答题 + 断线重连恢复
	if err := exam.SaveAnswer(ctx, database, sid, "L1-01-01", "A"); err != nil {
		t.Fatalf("答题失败: %v", err)
	}
	state, err := exam.Resume(ctx, database, sid, time.Now().UTC())
	if err != nil || state.Answers["L1-01-01"] != "A" || state.Expired {
		t.Fatalf("重连恢复失败: answers=%v expired=%v err=%v", state.Answers, state.Expired, err)
	}

	// 6. M3-02 交卷 + 客观自动判分
	res, err := exam.SubmitExam(ctx, database, sid, map[string]string{"L1-01-01": "A", "L1-01-02": "对"})
	if err != nil {
		t.Fatalf("交卷失败: %v", err)
	}
	if !res.IsPass || res.AutoScore != 3.0 {
		t.Fatalf("判分错误: is_pass=%v auto_score=%v", res.IsPass, res.AutoScore)
	}

	// 7. M3-06 发证 + M3-07 验真（交卷 → 判分 → 发证 → 验真 全链路）
	certNo, err := exam.IssueCertificate(ctx, database, exam.CertIssueInput{
		StudentID: stu.ID, CourseID: 1, CourseCode: "C1", Level: "L1", Score: res.TotalScore, Band: res.Band,
	})
	if err != nil {
		t.Fatalf("发证失败: %v", err)
	}
	vr, err := exam.VerifyCert(ctx, database, certNo, time.Now())
	if err != nil || vr.State != exam.StateValid {
		t.Fatalf("验真失败: state=%v err=%v", vr.State, err)
	}

	t.Logf("✅ 全链路通过：注册→登录→下单→mock支付(幂等)→权益→组卷→开考→答题→重连→交卷→判分→发证→验真（%v 分）", res.AutoScore)
}
