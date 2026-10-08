package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"

	"github.com/cw-tc/server/internal/config"
	"github.com/cw-tc/server/internal/identity"
	"github.com/cw-tc/server/internal/staff"
)

// seedResult 是单个种子账号的结果。
type seedResult struct {
	Account string
	Name    string
	Role    string
	ID      int64
	Created bool
}

// seedAdmin 实现 `cw-tc-server seed-admin` 的核心逻辑（可测试）。
//
// 一次种子三个角色（超管 / 运营 / 教师），每个账号密码都不写死默认值：
//
//	CWTC_ADMIN_ACCOUNT/PASSWORD   （admin_super，必填，缺失则失败）
//	CWTC_OPS_ACCOUNT/PASSWORD     （admin_ops，可选，缺密码则跳过）
//	CWTC_TEACHER_ACCOUNT/PASSWORD （teacher，可选，缺密码则跳过）
//
// 只连数据库（轻量配置，不校验身份证等强制密钥 —— 种子命令无需它们）。
// 返回 (结果列表, err)。
func seedAdmin(get config.Getenv) ([]seedResult, error) {
	accounts := []struct {
		acctKey, passKey, nameKey string
		defAcct, defName          string
		role                      staff.Role
		required                  bool
	}{
		{config.EnvAdminAccount, config.EnvAdminPassword, config.EnvAdminName, "admin", "超级管理员", staff.RoleAdminSuper, true},
		{config.EnvOpsAccount, config.EnvOpsPassword, config.EnvOpsName, "ops", "运营管理员", staff.RoleAdminOps, false},
		{config.EnvTeacherAccount, config.EnvTeacherPasswd, config.EnvTeacherName, "teacher", "教师", staff.RoleTeacher, false},
	}

	cfg := &config.Config{
		DeployEnv: strings.ToLower(strings.TrimSpace(get(config.EnvDeployEnv))),
		DBDriver:  strings.ToLower(strings.TrimSpace(get(config.EnvDBDriver))),
		DBDSN:     strings.TrimSpace(get(config.EnvDBDSN)),
	}
	if cfg.DeployEnv == "" {
		cfg.DeployEnv = "development"
	}
	if cfg.DBDriver == "" {
		cfg.DBDriver = "sqlite"
	}
	database, closeDB, err := openDatabase(context.Background(), cfg)
	if err != nil || database == nil {
		return nil, fmt.Errorf("数据库初始化失败: %w", err)
	}
	defer closeDB()

	results := make([]seedResult, 0, 3)
	for _, a := range accounts {
		account := strings.TrimSpace(get(a.acctKey))
		if account == "" {
			account = a.defAcct
		}
		name := strings.TrimSpace(get(a.nameKey))
		if name == "" {
			name = a.defName
		}
		password := get(a.passKey)
		if password == "" {
			if a.required {
				return nil, fmt.Errorf("初始超管密码未配置（%s）：密码绝不写死默认值", a.passKey)
			}
			slog.Info("跳过种子账号（密码未配置）", "account", account, "role", string(a.role))
			continue
		}
		id, created, err := staff.SeedStaff(context.Background(), database, account, name, a.role, password)
		if err != nil {
			return nil, fmt.Errorf("创建账号 %s 失败: %w", account, err)
		}
		results = append(results, seedResult{Account: account, Name: name, Role: string(a.role), ID: id, Created: created})
	}
	// 种子默认学员（可选）：手机号即登录账号（铁律 3），缺账号/密码则跳过。
	stu, err := seedStudent(context.Background(), database, get(config.EnvIDCardEncKey), get(config.EnvIDCardHMACKey), get)
	if err != nil {
		return nil, err
	}
	if stu != nil {
		results = append(results, *stu)
	}
	return results, nil
}

// seedStudent 种子默认学员：CWTC_STUDENT_ACCOUNT(手机号) + CWTC_STUDENT_PASSWORD。
// 缺账号或密码返回 nil（跳过）；缺加密密钥但已配置学员则报错（需真实密钥加密身份证号）。
// 身份证号缺省用固定样例号（铁律 3：样例号硬约束，校验位合法）。
func seedStudent(ctx context.Context, db *sql.DB, encKey, hmacKey string, get config.Getenv) (*seedResult, error) {
	phone := strings.TrimSpace(get(config.EnvStudentAccount))
	password := get(config.EnvStudentPassword)
	if phone == "" || password == "" {
		return nil, nil // 未配置，跳过
	}
	if encKey == "" || hmacKey == "" {
		return nil, fmt.Errorf("种子默认学员需配置身份证加密密钥（%s / %s）", config.EnvIDCardEncKey, config.EnvIDCardHMACKey)
	}
	name := strings.TrimSpace(get(config.EnvStudentName))
	if name == "" {
		name = "测试学员"
	}
	idCard := strings.TrimSpace(get(config.EnvStudentIDCard))
	if idCard == "" {
		idCard = "110101199001011237" // 固定样例号（校验位合法）
	}
	svc := identity.NewService(db, []byte(encKey), []byte(hmacKey))
	// 幂等：先查手机号是否已存在（student_no 是 phone+idcard 的哈希，重复注册会先撞 student_no 唯一约束，
	// 而非 phone 唯一约束，直接依赖 Register 的错误映射会漏判）。
	var existingID int64
	err := db.QueryRowContext(ctx, `SELECT id FROM student WHERE phone = $1`, phone).Scan(&existingID)
	if err == nil {
		return &seedResult{Account: phone, Name: name, Role: "student", ID: existingID, Created: false}, nil // 幂等跳过
	}
	if err != sql.ErrNoRows {
		return nil, err
	}
	stu, err := svc.Register(ctx, identity.RegisterInput{Name: name, Phone: phone, IDCard: idCard, Password: password})
	if err == identity.ErrPhoneRegistered {
		return &seedResult{Account: phone, Name: name, Role: "student", Created: false}, nil // 幂等跳过
	}
	if err != nil {
		return nil, fmt.Errorf("创建默认学员 %s 失败: %w", phone, err)
	}
	return &seedResult{Account: phone, Name: name, Role: "student", ID: stu.ID, Created: true}, nil
}

// runSeedAdmin 执行 seed-admin 子命令；错误通过返回值表达（由 main 决定退出码）。
func runSeedAdmin() error {
	results, err := seedAdmin(config.OSGetenv)
	if err != nil {
		slog.Error("种子命令失败", "err", err)
		return err
	}
	for _, r := range results {
		if r.Created {
			slog.Info("账号已创建（首登强制改密）", "account", r.Account, "name", r.Name, "role", r.Role, "id", r.ID)
		} else {
			slog.Info("账号已存在，幂等跳过", "account", r.Account, "role", r.Role, "id", r.ID)
		}
	}
	return nil
}
