// 命令 sync：把上游课程内容只读同步进数据库。
//
// 用法：
//
//	go run ./cmd/sync --source <上游目录> [--dsn <SQLite DSN>]
//
// 默认源目录取 CWTC_CONTENT_SOURCE，未设置时回退到本机上游路径。
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/cw-tc/server/internal/platform/db"
	"github.com/cw-tc/server/internal/sync"
	"github.com/cw-tc/server/migrations"
)

const defaultSource = "E:/2026-2027/2026-2027-1/AI化教学创新"

func main() {
	source := flag.String("source", os.Getenv("CWTC_CONTENT_SOURCE"), "上游内容目录（默认 CWTC_CONTENT_SOURCE）")
	dsn := flag.String("dsn", "", "SQLite DSN（默认内存库，仅开发/测试）")
	flag.Parse()

	courses, lectures, questions, err := runSync(context.Background(), *dsn, *source)
	if err != nil {
		log.Fatalf("同步失败: %v", err)
	}
	fmt.Printf("同步完成：上游课程 %d 门，讲次 %d 讲，题目 %d 题。\n", courses, lectures, questions)
}

// runSync 打开数据库、执行迁移并运行同步，返回上游课程数、讲次数与题目数。
func runSync(ctx context.Context, dsn, source string) (int, int, int, error) {
	if source == "" {
		source = defaultSource
	}
	database, err := db.Open(ctx, db.DialectSQLite, dsn)
	if err != nil {
		return 0, 0, 0, err
	}
	defer database.Close()
	if err := db.ApplyMigrations(ctx, database, migrations.FS, db.DialectSQLite); err != nil {
		return 0, 0, 0, err
	}
	if err := sync.Run(ctx, database, source); err != nil {
		return 0, 0, 0, err
	}
	var courses, lectures, questions int
	_ = database.QueryRow(`SELECT COUNT(*) FROM course WHERE source='upstream'`).Scan(&courses)
	_ = database.QueryRow(`SELECT COUNT(*) FROM lecture WHERE origin='upstream'`).Scan(&lectures)
	_ = database.QueryRow(`SELECT COUNT(*) FROM question`).Scan(&questions)
	return courses, lectures, questions, nil
}
