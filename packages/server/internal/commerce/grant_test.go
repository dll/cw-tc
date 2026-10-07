package commerce

import (
	"context"
	"testing"
)

// 铁律 8 · §7.3 G-7：授权免费学习 —— source='grant'，重复授权 409，撤销只收权益、已发证保留。
func TestGrantEntitlementAndRevoke(t *testing.T) {
	database := commerceDB(t)
	defer database.Close()
	service := NewService(database)
	ctx := context.Background()

	// 缺事由 → 拒绝
	if err := service.GrantEntitlement(ctx, 1, 1, 1, ""); err != ErrGrantInput {
		t.Fatalf("缺事由应报 ErrGrantInput，实际 %v", err)
	}
	// 授权成功
	if err := service.GrantEntitlement(ctx, 1, 1, 1, "校长特批"); err != nil {
		t.Fatal(err)
	}
	// 重复授权 → 拒绝
	if err := service.GrantEntitlement(ctx, 1, 1, 1, "再次授权"); err != ErrEntitlementExists {
		t.Fatalf("重复授权应报 ErrEntitlementExists，实际 %v", err)
	}
	// 授权后可学可考（权益存在）
	items, err := service.ListEntitlements(ctx, 1)
	if err != nil || len(items) != 1 || items[0].Source != "grant" {
		t.Fatalf("授权权益应存在: %d %v", len(items), err)
	}

	// 造一张证书，验证撤销授权后证书仍保留
	if _, err := database.Exec(`INSERT INTO certificate(cert_no, student_id, course_id, level, score, band, issued_at, expires_at) VALUES('CW-CN-A202600010',1,1,'L1',80,'pass','2026-01-01T00:00:00Z','2029-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	// 撤销授权
	if err := service.RevokeGrant(ctx, 1, 1); err != nil {
		t.Fatal(err)
	}
	// 权益已收
	items, _ = service.ListEntitlements(ctx, 1)
	if len(items) != 0 {
		t.Fatalf("撤销授权后权益应为空，实际 %d", len(items))
	}
	// 证书仍有效（撤销授权 ≠ 退款，不撤证）
	var status string
	_ = database.QueryRow(`SELECT status FROM certificate WHERE cert_no='CW-CN-A202600010'`).Scan(&status)
	if status != "valid" {
		t.Fatalf("撤销授权应保留证书，实际 %s", status)
	}
	// 再撤销 → 无权益
	if err := service.RevokeGrant(ctx, 1, 1); err != ErrEntitlementNotFound {
		t.Fatalf("无权益再撤销应报 ErrEntitlementNotFound，实际 %v", err)
	}
}
