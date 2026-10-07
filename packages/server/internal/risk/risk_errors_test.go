package risk

import (
	"context"
	"testing"
	"time"
)

func TestAddBlacklistInputErrors(t *testing.T) {
	ctx := context.Background()
	database := riskDB(t)
	defer database.Close()
	now := time.Now().UTC()

	if err := AddBlacklist(ctx, nil, TargetPhone, "13812345678", "x", now.Add(time.Hour), 0); err == nil {
		t.Error("nil db 应报错")
	}
	if err := AddBlacklist(ctx, database, TargetType("ip"), "1.1.1.1", "x", now.Add(time.Hour), 0); err != ErrBlacklistTarget {
		t.Errorf("非法粒度应报 ErrBlacklistTarget，实际 %v", err)
	}
	if err := AddBlacklist(ctx, database, TargetPhone, "", "x", now.Add(time.Hour), 0); err != ErrBlacklistInput {
		t.Errorf("空目标应报 ErrBlacklistInput，实际 %v", err)
	}
	if err := AddBlacklist(ctx, database, TargetPhone, "13812345678", "", now.Add(time.Hour), 0); err != ErrBlacklistEmpty {
		t.Errorf("空原因应报 ErrBlacklistEmpty，实际 %v", err)
	}
	if err := AddBlacklist(ctx, database, TargetPhone, "13812345678", "x", time.Time{}, 0); err != ErrBlacklistEmpty {
		t.Errorf("零到期时间应报 ErrBlacklistEmpty，实际 %v", err)
	}
}

func TestAppealAndResolveErrors(t *testing.T) {
	ctx := context.Background()
	database := riskDB(t)
	defer database.Close()

	if err := Appeal(ctx, database, 1, ""); err != ErrBlacklistInput {
		t.Errorf("空申诉理由应报 ErrBlacklistInput，实际 %v", err)
	}
	if err := Appeal(ctx, database, 999999, "申诉"); err == nil {
		t.Error("申诉不存在的条目应报错")
	}
	if err := ResolveAppeal(ctx, database, 999999, true); err == nil {
		t.Error("复核不存在的条目应报错")
	}
}

func TestRefundAbuseExceededNilQueryer(t *testing.T) {
	exceeded, err := RefundAbuseExceeded(context.Background(), nil, 1, 3, 365, time.Now())
	if err != nil || exceeded {
		t.Fatalf("nil queryer 应返回 false,nil：exceeded=%v err=%v", exceeded, err)
	}
}
