// 命令 sync：把上游课程内容只读同步进数据库。
//
// 用法：
//
//	go run ./cmd/sync --source <上游目录> [--driver sqlite|postgres] [--dsn <DSN>]
//
// 默认源目录取 CWTC_CONTENT_SOURCE，未设置时回退到本机上游路径；
// 默认驱动取 CWTC_DB_DRIVER（未设置回退 sqlite）。
package main

import (
	"context"
	"encoding/json"
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
	driver := flag.String("driver", os.Getenv("CWTC_DB_DRIVER"), "数据库驱动 sqlite|postgres（默认 CWTC_DB_DRIVER）")
	dsn := flag.String("dsn", os.Getenv("CWTC_DB_DSN"), "数据库 DSN（默认 CWTC_DB_DSN）")
	mediaManifest := flag.String("media-manifest", "", "媒体清单文件（JSON，写入 asset 表）")
	flag.Parse()

	courses, lectures, questions, err := runSync(context.Background(), *driver, *dsn, *source)
	if err != nil {
		log.Fatalf("同步失败: %v", err)
	}
	fmt.Printf("同步完成：上游课程 %d 门，讲次 %d 讲，题目 %d 题。\n", courses, lectures, questions)

	if *mediaManifest != "" {
		n, err := importMedia(context.Background(), *driver, *dsn, *mediaManifest)
		if err != nil {
			log.Fatalf("媒体清单导入失败: %v", err)
		}
		fmt.Printf("媒体资源导入 %d 条。\n", n)
	}
}

// importMedia 读取媒体清单 JSON 并写入 asset 表（视频/音频走 COS）。
func importMedia(ctx context.Context, driver, dsn, manifestPath string) (int, error) {
	dialect, err := db.ParseDialect(driver)
	if err != nil {
		return 0, err
	}
	database, err := db.Open(ctx, dialect, dsn)
	if err != nil {
		return 0, err
	}
	defer database.Close()
	if err := db.ApplyMigrations(ctx, database, migrations.FS, dialect); err != nil {
		return 0, err
	}
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		return 0, err
	}
	var entries []sync.MediaEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return 0, err
	}
	return sync.ImportMediaAssets(ctx, database, entries)
}

// runSync 打开数据库、执行迁移并运行同步，返回上游课程数、讲次数与题目数。
func runSync(ctx context.Context, driver, dsn, source string) (int, int, int, error) {
	if source == "" {
		source = defaultSource
	}
	dialect, err := db.ParseDialect(driver)
	if err != nil {
		return 0, 0, 0, err
	}
	database, err := db.Open(ctx, dialect, dsn)
	if err != nil {
		return 0, 0, 0, err
	}
	defer database.Close()
	if err := db.ApplyMigrations(ctx, database, migrations.FS, dialect); err != nil {
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
