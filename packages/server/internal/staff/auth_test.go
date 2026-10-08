package staff

import (
	"context"
	"testing"
	"time"
)

func TestStaffLoginAndAuthenticate(t *testing.T) {
	database := staffDB(t)
	defer database.Close()
	ctx := context.Background()
	now := time.Now().UTC()

	id, err := Create(ctx, database, "admin1", "管理员", RoleAdminOps, "Temp@123", 1)
	if err != nil {
		t.Fatal(err)
	}
	// 错误密码
	if _, _, err := Login(ctx, database, "admin1", "wrong", now); err != ErrInvalidCredentials {
		t.Fatalf("错误密码应报 ErrInvalidCredentials，实际 %v", err)
	}
	// 正确登录
	st, token, err := Login(ctx, database, "admin1", "Temp@123", now)
	if err != nil || st.Role != RoleAdminOps || token == "" {
		t.Fatalf("登录失败: %v %+v", err, st)
	}
	// 鉴权
	got, err := Authenticate(ctx, database, token, now)
	if err != nil || got.ID != id || got.Role != RoleAdminOps {
		t.Fatalf("鉴权失败: %v %+v", err, got)
	}
	// 无效 token
	if _, err := Authenticate(ctx, database, "deadbeef", now); err != ErrUnauthenticated {
		t.Fatalf("无效 token 应报 ErrUnauthenticated，实际 %v", err)
	}
	// 停用后登录被拒
	if err := Disable(ctx, database, id); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Login(ctx, database, "admin1", "Temp@123", now); err != ErrAccountDisabled {
		t.Fatalf("停用后登录应报 ErrAccountDisabled，实际 %v", err)
	}
	// 不存在的账号
	if _, _, err := Login(ctx, database, "nobody", "x", now); err != ErrInvalidCredentials {
		t.Fatalf("不存在账号应报 ErrInvalidCredentials，实际 %v", err)
	}
}

func TestStaffAuthenticateExpiredToken(t *testing.T) {
	database := staffDB(t)
	defer database.Close()
	ctx := context.Background()
	now := time.Now().UTC()

	id, _ := Create(ctx, database, "admin2", "管理员", RoleAdminOps, "Temp@123", 1)
	_, token, err := Login(ctx, database, "admin2", "Temp@123", now)
	if err != nil {
		t.Fatal(err)
	}
	_ = id
	// 把会话改为已过期
	if _, err := database.Exec(`UPDATE staff_session SET expires_at = $1`, now.Add(-1*time.Hour).UTC().Format(time.RFC3339)); err != nil {
		t.Fatal(err)
	}
	if _, err := Authenticate(ctx, database, token, now); err != ErrUnauthenticated {
		t.Fatalf("过期会话应报 ErrUnauthenticated，实际 %v", err)
	}
}
