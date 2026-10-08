package risk

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/cw-tc/server/internal/platform/db"
	"github.com/cw-tc/server/migrations"
)

func riskDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.Open(context.Background(), db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.ApplyMigrations(context.Background(), database, migrations.FS, db.DialectSQLite); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO student(id, student_no, name, phone, id_card_hash, id_card_cipher, agreement_versions) VALUES(1,'S00000001','样例学员','13812345678',X'01',X'02','[]')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO staff(id, account, name, role, password_hash, created_source) VALUES(1,'sample','样例管理员','admin_ops',X'01','seed')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO major(id, name) VALUES(1,'样例专业')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO course(id, course_code, source, name, line, major_id, status, teacher_id) VALUES(1,'C1','platform','样例课程','main',1,'published',1)`); err != nil {
		t.Fatal(err)
	}
	return database
}

func TestBlacklistAddCheckAndExpiry(t *testing.T) {
	database := riskDB(t)
	defer database.Close()
	ctx := context.Background()
	now := time.Now().UTC()

	if err := AddBlacklist(ctx, database, TargetPhone, "13812345678", "退款滥用", now.Add(90*24*time.Hour), 1); err != nil {
		t.Fatal(err)
	}
	hit, err := IsBlacklisted(ctx, database, TargetPhone, "13812345678", now)
	if err != nil || !hit {
		t.Fatalf("生效中的黑名单应命中: hit=%v err=%v", hit, err)
	}
	// 已过期 → 不命中
	miss, _ := IsBlacklisted(ctx, database, TargetPhone, "13812345678", now.Add(91*24*time.Hour))
	if miss {
		t.Fatal("已到期黑名单不应命中")
	}
}

func TestBlacklistAddSameTargetExpiresOld(t *testing.T) {
	database := riskDB(t)
	defer database.Close()
	ctx := context.Background()
	now := time.Now().UTC()

	if err := AddBlacklist(ctx, database, TargetDevice, "dev-1", "一", now.Add(90*24*time.Hour), 0); err != nil {
		t.Fatal(err)
	}
	if err := AddBlacklist(ctx, database, TargetDevice, "dev-1", "二", now.Add(120*24*time.Hour), 0); err != nil {
		t.Fatal(err)
	}
	var active, expired int
	_ = database.QueryRow(`SELECT COUNT(*) FROM risk_blacklist WHERE target_value='dev-1' AND status='active'`).Scan(&active)
	_ = database.QueryRow(`SELECT COUNT(*) FROM risk_blacklist WHERE target_value='dev-1' AND status='expired'`).Scan(&expired)
	if active != 1 || expired != 1 {
		t.Fatalf("同目标重复加黑应旧条转 expired，实际 active=%d expired=%d", active, expired)
	}
}

func TestBlacklistAppealSuspendsAndResolve(t *testing.T) {
	database := riskDB(t)
	defer database.Close()
	ctx := context.Background()
	now := time.Now().UTC()

	if err := AddBlacklist(ctx, database, TargetPaymentAccount, "acct-1", "违规", now.Add(90*24*time.Hour), 1); err != nil {
		t.Fatal(err)
	}
	var id int64
	_ = database.QueryRow(`SELECT id FROM risk_blacklist WHERE target_value='acct-1' AND status='active'`).Scan(&id)

	if err := Appeal(ctx, database, id, "我是正常用户"); err != nil {
		t.Fatal(err)
	}
	if hit, _ := IsBlacklisted(ctx, database, TargetPaymentAccount, "acct-1", now); hit {
		t.Fatal("申诉期间应暂不执行黑名单")
	}
	// 驳回 → 恢复执行
	if err := ResolveAppeal(ctx, database, id, false); err != nil {
		t.Fatal(err)
	}
	if hit, _ := IsBlacklisted(ctx, database, TargetPaymentAccount, "acct-1", now); !hit {
		t.Fatal("申诉被驳回应恢复执行")
	}
	// 接受 → 解除
	if err := Appeal(ctx, database, id, "再申诉"); err != nil {
		t.Fatal(err)
	}
	if err := ResolveAppeal(ctx, database, id, true); err != nil {
		t.Fatal(err)
	}
	if hit, _ := IsBlacklisted(ctx, database, TargetPaymentAccount, "acct-1", now); hit {
		t.Fatal("申诉被接受应解除黑名单")
	}
}

func seedRefund(t *testing.T, database *sql.DB, i int, initiator, kind string, finished time.Time) {
	t.Helper()
	orderNo := fmt.Sprintf("O%d", i)
	if _, err := database.Exec(
		`INSERT INTO orders(order_no, student_id, course_id, course_name_snapshot, price_snapshot_cents, amount_cents, status, expired_at) VALUES($1,1,1,'样例课程',1000,1000,'paid',$2)`,
		orderNo, finished.Add(1*time.Hour).UTC().Format(time.RFC3339)); err != nil {
		t.Fatal(err)
	}
	var orderID int64
	_ = database.QueryRow(`SELECT id FROM orders WHERE order_no=$1`, orderNo).Scan(&orderID)
	var basis, basisRef any
	if kind == "reward" {
		basis, basisRef = "campaign", "活动-1"
	}
	if _, err := database.Exec(
		`INSERT INTO refund(refund_no, order_id, kind, basis, basis_ref, amount_cents, reason, initiator, status, finished_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		fmt.Sprintf("R%d", i), orderID, kind, basis, basisRef, 1000, "事由", initiator, "success", finished.UTC().Format(time.RFC3339)); err != nil {
		t.Fatal(err)
	}
}

func TestRefundAbuseExceeded(t *testing.T) {
	database := riskDB(t)
	defer database.Close()
	ctx := context.Background()
	now := time.Now().UTC()

	exceeded, err := RefundAbuseExceeded(ctx, database, 1, 3, 365, now)
	if err != nil || exceeded {
		t.Fatalf("零退款不应超限: exceeded=%v err=%v", exceeded, err)
	}
	for i := 0; i < 3; i++ {
		seedRefund(t, database, i, "student", "normal", now.Add(-time.Duration(i)*24*time.Hour))
	}
	exceeded, err = RefundAbuseExceeded(ctx, database, 1, 3, 365, now)
	if err != nil || !exceeded {
		t.Fatalf("达 3 次应超限: exceeded=%v err=%v", exceeded, err)
	}
	// 奖励性退款（reward）不计入滥用
	seedRefund(t, database, 100, "student", "reward", now)
	exceeded, err = RefundAbuseExceeded(ctx, database, 1, 3, 365, now)
	if err != nil || !exceeded {
		t.Fatalf("reward 退款不应改变超限判断: exceeded=%v err=%v", exceeded, err)
	}
}
