package audit

import (
	"context"
	"testing"

	"github.com/cw-tc/server/internal/platform/db"
	"github.com/cw-tc/server/migrations"
)

func TestRecordAudit(t *testing.T) {
	database, err := db.Open(context.Background(), db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := db.ApplyMigrations(context.Background(), database, migrations.FS, db.DialectSQLite); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := Record(ctx, database, Input{ActorType: "staff", ActorID: 1, ActorRole: "admin_ops", Action: "user.reveal_phone", TargetType: "student", TargetID: "1"}); err != nil {
		t.Fatal(err)
	}
	var n int
	_ = database.QueryRow(`SELECT COUNT(*) FROM audit_log`).Scan(&n)
	if n != 1 {
		t.Fatalf("审计应写 1 条，实际 %d", n)
	}
	// 非法 actor_type
	if err := Record(ctx, database, Input{ActorType: "boss", ActorID: 1, Action: "x"}); err != ErrAuditInput {
		t.Fatalf("非法 actor_type 应报 ErrAuditInput，实际 %v", err)
	}
}
