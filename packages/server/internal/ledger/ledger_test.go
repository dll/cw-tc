package ledger

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"

	"github.com/cw-tc/server/internal/platform/db"
	"github.com/cw-tc/server/migrations"
)

func ledgerDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.Open(context.Background(), db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.ApplyMigrations(context.Background(), database, migrations.FS, db.DialectSQLite); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO student(id, student_no, name, phone, id_card_hash, id_card_cipher, agreement_versions) VALUES(1,'S0001','学员','13812345678',X'01',X'02','v1')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO staff(id, account, name, role, password_hash, created_source) VALUES(1,'t','教师','teacher',X'01','seed')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO major(id, name) VALUES(1,'样例专业')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO course(id, course_code, source, name, line, major_id, status, teacher_id) VALUES(1,'TC-1234','platform','课程','main',1,'published',1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO orders(id, order_no, student_id, course_id, course_name_snapshot, price_snapshot_cents, amount_cents, status, channel, expired_at, paid_at) VALUES(1,'O1',1,1,'课程',100,100,'paid','wechat','2026-01-01T00:00:00Z','2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO refund(id, refund_no, order_id, kind, amount_cents, reason, initiator, status) VALUES(1,'R1',1,'normal',100,'退款','student','success')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO certificate(id, cert_no, student_id, course_id, level, score, band, issued_at, expires_at, status) VALUES(1,'CW-CN-A202600001',1,1,'L1',88,'pass','2026-01-01T00:00:00Z','2029-01-01T00:00:00Z','valid')`); err != nil {
		t.Fatal(err)
	}
	return database
}

func TestLedgersMaskPhone(t *testing.T) {
	database := ledgerDB(t)
	defer database.Close()
	ctx := context.Background()

	orders, err := ListOrders(ctx, database, "paid", 10, 0)
	if err != nil || len(orders) != 1 {
		t.Fatalf("订单台账应 1 条: %d %v", len(orders), err)
	}
	if orders[0].Phone != "138****5678" {
		t.Fatalf("手机号应脱敏为 138****5678，实际 %s", orders[0].Phone)
	}

	refunds, err := ListRefunds(ctx, database, 10, 0)
	if err != nil || len(refunds) != 1 || refunds[0].Phone != "138****5678" {
		t.Fatalf("退款台账脱敏失败: %+v %v", refunds, err)
	}

	certs, err := ListCertificates(ctx, database, 10, 0)
	if err != nil || len(certs) != 1 || certs[0].Course != "TC-1234" || certs[0].Phone != "138****5678" {
		t.Fatalf("证书台账失败: %+v %v", certs, err)
	}
}

// TestLedgerJSONTags 锁台账 JSON 字段名为 snake_case（前端 client.ts 的 OrderItem 等期望），
// 否则管理端金额列取不到 amount_cents → 显示 NaN、其余列空。
func TestLedgerJSONTags(t *testing.T) {
	if o, _ := json.Marshal(OrderRow{OrderNo: "O1", AmountCents: 100}); !strings.Contains(string(o), `"order_no"`) || !strings.Contains(string(o), `"amount_cents"`) {
		t.Fatalf("OrderRow 应序列化为 snake_case: %s", o)
	}
	if r, _ := json.Marshal(RefundRow{RefundNo: "R1", AmountCents: 100}); !strings.Contains(string(r), `"refund_no"`) || !strings.Contains(string(r), `"amount_cents"`) {
		t.Fatalf("RefundRow 应序列化为 snake_case: %s", r)
	}
	if c, _ := json.Marshal(CertRow{CertNo: "CW", Score: 88}); !strings.Contains(string(c), `"cert_no"`) || !strings.Contains(string(c), `"score"`) {
		t.Fatalf("CertRow 应序列化为 snake_case: %s", c)
	}
}
