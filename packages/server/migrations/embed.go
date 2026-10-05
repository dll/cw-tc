// Package migrations 用 embed 提供两套方言的迁移脚本。
//
//	postgres/ —— **生产唯一权威**，与 `docs/数据字典-v1.md` §8 的切分一一对应；
//	sqlite/   —— **开发与测试使用**（由 postgres/ 机械翻译，表与列一一对应）。
//
// 运行时**不做任何方言翻译**：落盘的就是两套静态 SQL，可逐行 review，
// 并由 `migrations_test.go` 的结构一致性门禁保证两者不会悄悄分叉。
package migrations

import "embed"

//go:embed postgres sqlite
var FS embed.FS
