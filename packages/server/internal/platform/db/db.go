// Package db 提供数据库连接池与迁移执行。
//
// # 数据库边界（2026-10-06 明确）
//
//   - **开发与测试：SQLite**（纯 Go 驱动 `modernc.org/sqlite`，无需 CGO），
//     **不依赖本机 PostgreSQL / Redis**；
//   - **生产：腾讯云 PostgreSQL**，沿用 CCIT 的服务器、部署与环境变量约定；
//   - **不新建服务器 / 数据库 / 本机服务**；**不读取或改动 CCIT 的真实生产数据**；
//   - 本机曾有的 PostgreSQL 只作"存在性探测"，**后续不再使用**。
//
// # 一条纪律
//
// **生产不得使用 SQLite**（`config` 层在 `DEPLOY_ENV=production` 时拒绝该驱动）；
// PostgreSQL 驱动**本仓库尚未接入** —— 本机没有可验证的实例，宁可显式失败，
// 也不引入一个无法验证的依赖（见 `ErrDialectUnsupported`）。
package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

// Dialect 数据库方言。取值即 migrations/ 下的子目录名。
type Dialect string

const (
	DialectSQLite   Dialect = "sqlite"
	DialectPostgres Dialect = "postgres"
)

var (
	// ErrNotConfigured 未配置数据库。
	ErrNotConfigured = errors.New("数据库未配置")
	// ErrDialectUnsupported 该方言尚未接入驱动（当前仅 SQLite）。
	ErrDialectUnsupported = errors.New("该数据库方言尚未接入驱动")
	// ErrUnknownDialect 未知方言。
	ErrUnknownDialect = errors.New("未知的数据库方言")
)

// Pinger / Closer / Pool：供不依赖具体驱动的调用方使用。
type Pinger interface{ Ping(context.Context) error }
type Closer interface{ Close() error }
type Pool interface {
	Pinger
	Closer
}

// Unconfigured 是"未配置数据库"时的 fail-closed 实现：调用必返回 ErrNotConfigured。
type Unconfigured struct{}

func (Unconfigured) Ping(context.Context) error { return ErrNotConfigured }
func (Unconfigured) Close() error               { return nil }

// ParseDialect 解析方言字符串。
func ParseDialect(s string) (Dialect, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", string(DialectSQLite):
		return DialectSQLite, nil
	case string(DialectPostgres):
		return DialectPostgres, nil
	default:
		return "", fmt.Errorf("%w：%q", ErrUnknownDialect, s)
	}
}

// Open 按方言打开连接池。
//
//   - sqlite：`dsn` 为空时使用**共享内存库**（测试用）；并显式打开 `foreign_keys`，
//     使外键约束行为与 PostgreSQL 对齐（SQLite 默认**不**enforce 外键）。
//   - postgres：**尚未接入驱动**，返回 ErrDialectUnsupported（显式失败，不静默降级）。
func Open(ctx context.Context, dialect Dialect, dsn string) (*sql.DB, error) {
	switch dialect {
	case DialectSQLite:
		return openSQLite(ctx, dsn)
	case DialectPostgres:
		// 刻意留空：本机没有可验证的 PG 实例，接入未验证的驱动不如显式失败。
		// 接入点见 docs/技术方案-v1.md §5 与批次 1 任务（有生产实例可验时再做）。
		return nil, fmt.Errorf("%w：postgres（开发/测试请用 sqlite；生产接入见批次 1）", ErrDialectUnsupported)
	default:
		return nil, fmt.Errorf("%w：%q", ErrUnknownDialect, dialect)
	}
}

func openSQLite(ctx context.Context, dsn string) (*sql.DB, error) {
	if strings.TrimSpace(dsn) == "" {
		// 共享内存库 + 外键开启（_pragma 由驱动在每个连接建立时应用）。
		dsn = "file::memory:?cache=shared&_pragma=foreign_keys(1)"
	}
	database, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// 内存库在多个连接下易出现表锁；单连接即可满足开发与测试。
	database.SetMaxOpenConns(1)
	if err := database.PingContext(ctx); err != nil {
		_ = database.Close()
		return nil, err
	}
	return database, nil
}

// ApplyMigrations 执行 fsys 中 `<dialect>/` 子目录下的迁移，按文件名升序、逐个事务。
//
// 幂等：已记录在 `schema_migrations` 的文件会跳过；重跑安全。
func ApplyMigrations(ctx context.Context, database *sql.DB, fsys fs.FS, dialect Dialect) error {
	if database == nil {
		return ErrNotConfigured
	}
	dir := string(dialect)
	if dir != string(DialectSQLite) && dir != string(DialectPostgres) {
		return fmt.Errorf("%w：%q", ErrUnknownDialect, dialect)
	}

	if _, err := database.ExecContext(ctx,
		`CREATE TABLE IF NOT EXISTS schema_migrations (`+
			`version TEXT PRIMARY KEY, `+
			`applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP)`); err != nil {
		return fmt.Errorf("创建 schema_migrations 失败：%w", err)
	}

	files, err := sqlFiles(fsys, dir)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("迁移目录 %s/ 下没有 .sql（embed 是否漏了该目录？）", dir)
	}

	for _, name := range files {
		applied, err := migrationApplied(ctx, database, name)
		if err != nil {
			return err
		}
		if applied {
			continue
		}
		if err := applyOne(ctx, database, fsys, dir, name); err != nil {
			return err
		}
	}
	return nil
}

func sqlFiles(fsys fs.FS, dir string) ([]string, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("读取迁移目录 %s/ 失败：%w", dir, err)
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)
	return files, nil
}

func migrationApplied(ctx context.Context, database *sql.DB, version string) (bool, error) {
	var n int
	if err := database.QueryRowContext(ctx,
		`SELECT COUNT(1) FROM schema_migrations WHERE version = ?`, version).Scan(&n); err != nil {
		// PostgreSQL 的占位符是 $1；这里按方言回退一次（SQLite 用 ?）。
		if err2 := database.QueryRowContext(ctx,
			`SELECT COUNT(1) FROM schema_migrations WHERE version = $1`, version).Scan(&n); err2 != nil {
			return false, fmt.Errorf("查询迁移记录失败：%w", err)
		}
	}
	return n > 0, nil
}

func applyOne(ctx context.Context, database *sql.DB, fsys fs.FS, dir, name string) error {
	body, err := fs.ReadFile(fsys, dir+"/"+name)
	if err != nil {
		return fmt.Errorf("读取 %s/%s 失败：%w", dir, name, err)
	}
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, string(body)); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("执行迁移 %s/%s 失败：%w", dir, name, err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO schema_migrations(version) VALUES (?)`, name); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("记录迁移 %s 失败：%w", name, err)
	}
	return tx.Commit()
}
