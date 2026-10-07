package db

import (
	"context"
	"errors"
	"testing"
	"testing/fstest"
)

func TestOpenSQLiteInMemory(t *testing.T) {
	ctx := context.Background()
	database, err := Open(ctx, DialectSQLite, "")
	if err != nil {
		t.Fatalf("打开 SQLite 失败：%v", err)
	}
	if err := database.PingContext(ctx); err != nil {
		t.Fatalf("Ping 失败：%v", err)
	}
	// 外键必须已开启（SQLite 默认不 enforce，与 PostgreSQL 行为不一致）。
	var fk int
	if err := database.QueryRowContext(ctx, `PRAGMA foreign_keys`).Scan(&fk); err != nil {
		t.Fatalf("查询 foreign_keys 失败：%v", err)
	}
	if fk != 1 {
		t.Errorf("foreign_keys = %d，期望 1（Open 必须显式开启，否则约束行为与生产不一致）", fk)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("关闭失败：%v", err)
	}
}

func TestOpenSQLiteWithDSN(t *testing.T) {
	database, err := Open(context.Background(), DialectSQLite, "file:test-dsn?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("带 DSN 打开 SQLite 失败: %v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("关闭 SQLite 失败: %v", err)
	}
}

func TestOpenPostgresReachesDriver(t *testing.T) {
	// 生产走 PostgreSQL，驱动已接入（lib/pq）。这里断言：
	// 1) 不静默降级到其它方言（不应返回 ErrUnknownDialect / ErrDialectUnsupported）；
	// 2) 不可达的 DSN 会显式报连接错误（PingContext 失败），而不是假装成功。
	_, err := Open(context.Background(), DialectPostgres, "postgres://x:1/nope?connect_timeout=1")
	if err == nil {
		t.Fatal("不可达的 postgres DSN 应报错，实际成功")
	}
	if errors.Is(err, ErrUnknownDialect) || errors.Is(err, ErrDialectUnsupported) {
		t.Fatalf("postgres 已接入驱动，不应返回方言不支持错误，实际：%v", err)
	}
}

func TestOpenUnknownDialect(t *testing.T) {
	if _, err := Open(context.Background(), Dialect("mysql"), ""); !errors.Is(err, ErrUnknownDialect) {
		t.Fatalf("未知方言应返回 ErrUnknownDialect，实际：%v", err)
	}
}

func TestParseDialect(t *testing.T) {
	cases := map[string]Dialect{"": DialectSQLite, "sqlite": DialectSQLite,
		"SQLite": DialectSQLite, " postgres ": DialectPostgres}
	for in, want := range cases {
		got, err := ParseDialect(in)
		if err != nil {
			t.Fatalf("ParseDialect(%q) 报错：%v", in, err)
		}
		if got != want {
			t.Errorf("ParseDialect(%q) = %q，期望 %q", in, got, want)
		}
	}
	if _, err := ParseDialect("oracle"); !errors.Is(err, ErrUnknownDialect) {
		t.Errorf("ParseDialect(oracle) 应报错，实际：%v", err)
	}
}

func TestUnconfiguredFailsClosed(t *testing.T) {
	var u Unconfigured
	if err := u.Ping(context.Background()); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("未配置时 Ping 应返回 ErrNotConfigured，实际：%v", err)
	}
	if err := u.Close(); err != nil {
		t.Errorf("Close 应可重复调用且无错，实际：%v", err)
	}
}

func TestApplyMigrationsIsIdempotent(t *testing.T) {
	ctx := context.Background()
	database, err := Open(ctx, DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	fsys := fstest.MapFS{
		"sqlite/0002_test.sql": &fstest.MapFile{Data: []byte("CREATE TABLE IF NOT EXISTS demo (id INTEGER PRIMARY KEY);")},
		"sqlite/0003_test.sql": &fstest.MapFile{Data: []byte("CREATE TABLE IF NOT EXISTS demo_two (id INTEGER PRIMARY KEY);")},
	}
	for i := 0; i < 2; i++ {
		if err := ApplyMigrations(ctx, database, fsys, DialectSQLite); err != nil {
			t.Fatalf("第 %d 次执行迁移失败：%v", i+1, err)
		}
	}
	var n int
	if err := database.QueryRowContext(ctx, `SELECT COUNT(1) FROM schema_migrations`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("迁移记录数 = %d，期望 2（重跑不应重复记录）", n)
	}
}

func TestApplyMigrationsEmptyDirFailsLoudly(t *testing.T) {
	// 目录为空时必须**报错**，而不是安静地"迁移完成"。
	// （这是本项目反复踩过的一类问题：门禁/脚本退出码为 0，其实什么都没做。）
	ctx := context.Background()
	database, err := Open(ctx, DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	fsys := fstest.MapFS{"sqlite/readme.txt": &fstest.MapFile{Data: []byte("no sql here")}}
	if err := ApplyMigrations(ctx, database, fsys, DialectSQLite); err == nil {
		t.Fatal("目录下没有 .sql 时应报错，实际返回 nil（静默通过）")
	}
}

func TestApplyMigrationsNilDB(t *testing.T) {
	if err := ApplyMigrations(context.Background(), nil, fstest.MapFS{}, DialectSQLite); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("database 为 nil 时应返回 ErrNotConfigured，实际：%v", err)
	}
}
