package migrations_test

import (
	"context"
	"database/sql"
	"io/fs"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/cw-tc/server/internal/platform/db"
	"github.com/cw-tc/server/migrations"
)

// dialectException：**允许只在某一种方言里存在的迁移**，必须写明理由。
// 显式登记本身就是一次人工确认 —— 不做"自动放行"。
var dialectException = map[string]string{
	"0001_enable_extensions.sql": "pgvector 扩展仅 PostgreSQL 有；SQLite 侧不建向量索引（开发/测试走暴力检索兜底）",
}

// ---------------------------------------------------------------------------
// 1) SQLite 那一套**必须真的能跑**（不是查字符串）
// ---------------------------------------------------------------------------

func TestSQLiteMigrationsExecuteAndAreIdempotent(t *testing.T) {
	ctx := context.Background()
	database, err := db.Open(ctx, db.DialectSQLite, "")
	if err != nil {
		t.Fatalf("打开 SQLite 失败：%v", err)
	}
	defer database.Close()

	if err := db.ApplyMigrations(ctx, database, migrations.FS, db.DialectSQLite); err != nil {
		t.Fatalf("首次执行 SQLite 迁移失败：%v", err)
	}

	// 幂等：再跑一遍必须无副作用（跳过已记录的文件）。
	before := countMigrations(t, database)
	if err := db.ApplyMigrations(ctx, database, migrations.FS, db.DialectSQLite); err != nil {
		t.Fatalf("重复执行 SQLite 迁移失败：%v", err)
	}
	if after := countMigrations(t, database); after != before {
		t.Fatalf("重跑后迁移记录数变化：%d -> %d（不幂等）", before, after)
	}

	// 关键表必须真的建出来了。
	want := []string{
		"student", "staff", "student_session", "sms_code",
		"major", "course", "lecture", "asset", "question",
		"entitlement", "orders", "payment_txn", "refund", "reconcile_record",
		"promo_code", "promo_redemption",
		"exam_session", "exam_result", "manual_review", "cert_no_sequence", "certificate",
	}
	for _, table := range want {
		var n int
		if err := database.QueryRowContext(ctx,
			`SELECT COUNT(1) FROM sqlite_master WHERE type='table' AND name = ?`, table).Scan(&n); err != nil {
			t.Fatalf("查询表 %s 失败：%v", table, err)
		}
		if n != 1 {
			t.Errorf("SQLite 里缺少表 %s（迁移未真正生效？）", table)
		}
	}

	// 部分唯一索引：SQLite 支持，必须建出来（它承载"未撤销证书唯一 / 有效权益唯一 / 防重复下单"等口径）。
	// ⚠️ `uq_exam_ongoing`（同档单会话）按数据字典属 **v2.16 → 迁移 0019**，**不在本批次**，
	//    故此处故意不列（由后续迁移补，见 0005 文件里的 `@deferred` 行）。
	for _, idx := range []string{"uq_certificate_active", "uq_entitlement_active", "uq_pending_order"} {
		var n int
		if err := database.QueryRowContext(ctx,
			`SELECT COUNT(1) FROM sqlite_master WHERE type='index' AND name = ?`, idx).Scan(&n); err != nil {
			t.Fatalf("查询索引 %s 失败：%v", idx, err)
		}
		if n != 1 {
			t.Errorf("SQLite 里缺少部分唯一索引 %s", idx)
		}
	}

	// 外键必须真的被 enforce（SQLite 默认不 enforce；Open 里已开 foreign_keys）。
	if _, err := database.ExecContext(ctx,
		`INSERT INTO student_session(student_id, token_hash, issued_at, expires_at) VALUES (999999, X'00', 't', 't')`); err == nil {
		t.Error("外键未生效：插入了不存在的 student_id 却成功了（foreign_keys 未开启？）")
	}
}

func countMigrations(t *testing.T, database *sql.DB) int {
	t.Helper()
	var n int
	if err := database.QueryRow(`SELECT COUNT(1) FROM schema_migrations`).Scan(&n); err != nil {
		t.Fatalf("统计 schema_migrations 失败：%v", err)
	}
	return n
}

// ---------------------------------------------------------------------------
// 2) 两套方言的结构一致性（表与列必须一一对应）
// ---------------------------------------------------------------------------

func TestDialectSchemasMatch(t *testing.T) {
	pgFiles := sqlFilesIn(t, "postgres")
	liteFiles := sqlFilesIn(t, "sqlite")

	// 文件清单：去掉显式豁免后必须完全一致（顺序也一致）。
	want := withoutExceptions(pgFiles)
	if got := withoutExceptions(liteFiles); !equalStrings(want, got) {
		t.Fatalf("两套方言的迁移文件不一致：\n  postgres = %v\n  sqlite   = %v", want, got)
	}
	for _, name := range pgFiles {
		if _, ok := dialectException[name]; ok {
			continue
		}
		if !contains(liteFiles, name) {
			t.Errorf("sqlite/ 缺少 %s（若确实不适用，请登记到 dialectException 并写明理由）", name)
		}
	}

	// 表与列：逐文件比对。
	for _, name := range liteFiles {
		pgBody := readMigration(t, "postgres", name)
		liteBody := readMigration(t, "sqlite", name)
		pgTables := parseTables(pgBody)
		liteTables := parseTables(liteBody)
		if len(pgTables) == 0 {
			t.Errorf("postgres/%s 里没解析到 CREATE TABLE（解析器与写法不符？）", name)
			continue
		}
		for table, pgCols := range pgTables {
			liteCols, ok := liteTables[table]
			if !ok {
				t.Errorf("%s：postgres 有表 %s，sqlite 没有", name, table)
				continue
			}
			if !equalStrings(pgCols, liteCols) {
				t.Errorf("%s：表 %s 的列不一致\n  postgres = %v\n  sqlite   = %v", name, table, pgCols, liteCols)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// 3) 方言 lint：不许把另一边的专有语法写进来（手工编辑最容易犯）
// ---------------------------------------------------------------------------

func TestDialectLint(t *testing.T) {
	pgForbidden := map[string]string{
		"AUTOINCREMENT": "AUTOINCREMENT 是 SQLite 写法，只能出现在 sqlite/",
		"PRAGMA":        "PRAGMA 是 SQLite 写法",
		"BLOB":          "BLOB 是 SQLite 写法，PostgreSQL 用 BYTEA",
	}
	liteForbidden := map[string]string{
		"BIGSERIAL":        "BIGSERIAL 是 PostgreSQL 写法，SQLite 用 INTEGER PRIMARY KEY AUTOINCREMENT",
		"TIMESTAMPTZ":      "TIMESTAMPTZ 是 PostgreSQL 写法，SQLite 用 TEXT",
		"BYTEA":            "BYTEA 是 PostgreSQL 写法，SQLite 用 BLOB",
		"JSONB":            "JSONB 是 PostgreSQL 写法，SQLite 用 TEXT",
		"::jsonb":          "PG 的类型转换语法，SQLite 不支持",
		"DEFAULT now()":    "now() 是 PostgreSQL 函数，SQLite 用 CURRENT_TIMESTAMP",
		"CREATE EXTENSION": "扩展是 PostgreSQL 概念",
	}

	for _, name := range sqlFilesIn(t, "postgres") {
		if _, ok := dialectException[name]; ok {
			continue
		}
		body := strings.ToUpper(stripSQLComments(readMigration(t, "postgres", name)))
		for token, why := range pgForbidden {
			if strings.Contains(body, token) {
				t.Errorf("postgres/%s 含 SQLite 专有语法 %s：%s", name, token, why)
			}
		}
	}
	for _, name := range sqlFilesIn(t, "sqlite") {
		body := strings.ToUpper(stripSQLComments(readMigration(t, "sqlite", name)))
		for token, why := range liteForbidden {
			if strings.Contains(body, token) {
				t.Errorf("sqlite/%s 含 PostgreSQL 专有语法 %s：%s", name, token, why)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// 4) 幂等声明与密钥材料
// ---------------------------------------------------------------------------

func TestMigrationsAreIdempotentDDLAndContainNoSecrets(t *testing.T) {
	forbidden := []string{"CREATE ROLE", "CREATE USER", "PASSWORD '", "SECRET '", "PRIVATE KEY"}
	for _, dialect := range []string{"postgres", "sqlite"} {
		for _, name := range sqlFilesIn(t, dialect) {
			body := readMigration(t, dialect, name)
			upper := strings.ToUpper(body)
			if !strings.Contains(upper, "IF NOT EXISTS") {
				t.Errorf("%s/%s 缺少幂等 DDL：IF NOT EXISTS", dialect, name)
			}
			for _, bad := range forbidden {
				if strings.Contains(upper, bad) {
					t.Errorf("%s/%s 不得包含密钥或密码材料：%s", dialect, name, bad)
				}
			}
		}
	}
}

// =========================== 工具函数 ===========================

func sqlFilesIn(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := fs.ReadDir(migrations.FS, dir)
	if err != nil {
		t.Fatalf("读取 %s/ 失败：%v", dir, err)
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	if len(out) == 0 {
		t.Fatalf("%s/ 下没有 .sql（embed 漏了？）", dir)
	}
	return out
}

func readMigration(t *testing.T, dialect, name string) string {
	t.Helper()
	b, err := fs.ReadFile(migrations.FS, dialect+"/"+name)
	if err != nil {
		t.Fatalf("读取 %s/%s 失败：%v", dialect, name, err)
	}
	return string(b)
}

var tableHeadRe = regexp.MustCompile(`(?is)CREATE TABLE (?:IF NOT EXISTS )?([a-z_][a-z0-9_]*)\s*\(`)
var columnRe = regexp.MustCompile(`^\s*([a-z_][a-z0-9_]*)\s+`)

func stripSQLComments(s string) string {
	lines := strings.Split(s, "\n")
	out := lines[:0]
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// 只取列定义行；跳过表级约束（CHECK / UNIQUE / PRIMARY / FOREIGN / CONSTRAINT）。
func parseTables(sqlText string) map[string][]string {
	out := map[string][]string{}
	locs := tableHeadRe.FindAllStringSubmatchIndex(sqlText, -1)
	for i, loc := range locs {
		name := sqlText[loc[2]:loc[3]]
		start := loc[1]
		end := len(sqlText)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		block := sqlText[start:end]
		if cut := strings.Index(block, ");"); cut >= 0 {
			block = block[:cut]
		}
		var cols []string
		for _, line := range strings.Split(block, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "--") {
				continue
			}
			upper := strings.ToUpper(trimmed)
			if strings.HasPrefix(upper, "CHECK") || strings.HasPrefix(upper, "UNIQUE") ||
				strings.HasPrefix(upper, "PRIMARY") || strings.HasPrefix(upper, "FOREIGN") ||
				strings.HasPrefix(upper, "CONSTRAINT") {
				continue
			}
			if m := columnRe.FindStringSubmatch(trimmed); m != nil {
				cols = append(cols, m[1])
			}
		}
		sort.Strings(cols)
		out[name] = cols
	}
	return out
}

func withoutExceptions(files []string) []string {
	var out []string
	for _, f := range files {
		if _, ok := dialectException[f]; !ok {
			out = append(out, f)
		}
	}
	return out
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
