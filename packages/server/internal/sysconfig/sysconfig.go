// Package sysconfig 读取 / 修改平台业务参数（数据字典 §6.7）。
//
// 三条硬约束（AGENTS 铁律 11）：
//   - 只放业务参数，密钥只走 shared/.env；
//   - 取值优先级：环境变量 > system_config > 代码默认值；
//   - 展示阈值下限 3，后台改坏也必须被拒。
package sysconfig

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	KeyStatsDisplayThreshold   = "stats.display_threshold"
	KeyCommentDisplayThreshold = "comment.display_threshold"
	KeyOfflineValidDays        = "offline.valid_days"

	DefaultStatsDisplayThreshold   = 10
	DefaultCommentDisplayThreshold = 10
	DefaultOfflineValidDays        = 30

	MinDisplayThreshold = 3
	MaxOfflineValidDays = 365
)

var (
	ErrUnknownKey = errors.New("未知配置项")
	ErrBadValue   = errors.New("配置值不合法")
)

// Item 是后台系统配置页展示的一行。
type Item struct {
	Key         string `json:"key"`
	Value       any    `json:"value"`
	ValueType   string `json:"value_type"`
	MinValue    *int64 `json:"min_value"`
	MaxValue    *int64 `json:"max_value"`
	Description string `json:"description"`
	UpdatedBy   *int64 `json:"updated_by"`
	UpdatedAt   string `json:"updated_at"`
}

type execer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

// Store 基于 system_config 表。
type Store struct{ db *sql.DB }

func New(db *sql.DB) *Store { return &Store{db: db} }

// Int 读整数配置：环境变量 > 表 > 默认值。任何异常都回退默认值（读配置不应打断业务）。
func (s *Store) Int(ctx context.Context, key string, fallback int) int {
	if raw, ok := envValue(key); ok {
		if n, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil {
			return n
		}
	}
	if s == nil || s.db == nil {
		return fallback
	}
	var raw string
	if err := s.db.QueryRowContext(ctx, `SELECT value FROM system_config WHERE key = $1`, key).Scan(&raw); err != nil {
		return fallback
	}
	n, err := parseIntValue(raw)
	if err != nil {
		return fallback
	}
	return n
}

// List 返回全部业务参数（不含任何密钥；system_config 本身就不应存密钥）。
func (s *Store) List(ctx context.Context) ([]Item, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("数据库未配置")
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT key, value, value_type, min_value, max_value, description, updated_by, updated_at
		 FROM system_config ORDER BY key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Item, 0)
	for rows.Next() {
		item, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// Get 读取单个配置项（不存在返回 ErrUnknownKey）。
func (s *Store) Get(ctx context.Context, key string) (Item, error) {
	if s == nil || s.db == nil {
		return Item{}, errors.New("数据库未配置")
	}
	row := s.db.QueryRowContext(ctx,
		`SELECT key, value, value_type, min_value, max_value, description, updated_by, updated_at
		 FROM system_config WHERE key = $1`, key)
	item, err := scanItem(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Item{}, ErrUnknownKey
	}
	return item, err
}

// Set 修改一个业务参数（逐个 key 写；写前按 min_value / max_value 校验）。
func (s *Store) Set(ctx context.Context, key string, value any, updatedBy int64) (Item, error) {
	if s == nil || s.db == nil {
		return Item{}, errors.New("数据库未配置")
	}
	if updatedBy <= 0 || !KnownKey(key) {
		return Item{}, ErrUnknownKey
	}
	if SecretLike(key) {
		return Item{}, ErrUnknownKey
	}
	var valueType, description string
	var minValue, maxValue sql.NullInt64
	if err := s.db.QueryRowContext(ctx,
		`SELECT value_type, min_value, max_value, description FROM system_config WHERE key = $1`, key).
		Scan(&valueType, &minValue, &maxValue, &description); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Item{}, ErrUnknownKey
		}
		return Item{}, err
	}
	n, err := coerceInt(valueType, value, minValue, maxValue)
	if err != nil {
		return Item{}, err
	}
	encoded, _ := json.Marshal(n)
	if _, err := s.db.ExecContext(ctx,
		`UPDATE system_config SET value = $1, updated_by = $2, updated_at = $3 WHERE key = $4`,
		string(encoded), updatedBy, time.Now().UTC().Format(time.RFC3339), key); err != nil {
		return Item{}, err
	}
	return Item{Key: key, Value: int(n), ValueType: valueType, MinValue: nullableInt64(minValue),
		MaxValue: nullableInt64(maxValue), Description: description,
		UpdatedBy: &updatedBy, UpdatedAt: time.Now().UTC().Format(time.RFC3339)}, nil
}

func scanItem(scanner interface{ Scan(...any) error }) (Item, error) {
	var item Item
	var raw string
	var minValue, maxValue sql.NullInt64
	var updatedBy sql.NullInt64
	var updatedAt string
	if err := scanner.Scan(&item.Key, &raw, &item.ValueType, &minValue, &maxValue,
		&item.Description, &updatedBy, &updatedAt); err != nil {
		return Item{}, err
	}
	switch item.ValueType {
	case "int":
		value, err := parseIntValue(raw)
		if err != nil {
			return Item{}, err
		}
		item.Value = value
	case "bool":
		var b bool
		if err := json.Unmarshal([]byte(raw), &b); err == nil {
			item.Value = b
		} else {
			item.Value = strings.EqualFold(strings.TrimSpace(raw), "true")
		}
	default:
		var value any
		if err := json.Unmarshal([]byte(raw), &value); err != nil {
			value = raw
		}
		item.Value = value
	}
	item.MinValue = nullableInt64(minValue)
	item.MaxValue = nullableInt64(maxValue)
	if updatedBy.Valid {
		v := updatedBy.Int64
		item.UpdatedBy = &v
	}
	item.UpdatedAt = updatedAt
	return item, nil
}

func coerceInt(valueType string, value any, minValue, maxValue sql.NullInt64) (int64, error) {
	if valueType != "int" {
		return 0, ErrBadValue
	}
	var n int64
	switch v := value.(type) {
	case int:
		n = int64(v)
	case int64:
		n = v
	case float64:
		if v != float64(int64(v)) {
			return 0, ErrBadValue
		}
		n = int64(v)
	case json.Number:
		parsed, err := v.Int64()
		if err != nil {
			return 0, ErrBadValue
		}
		n = parsed
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		if err != nil {
			return 0, ErrBadValue
		}
		n = parsed
	default:
		return 0, ErrBadValue
	}
	if minValue.Valid && n < minValue.Int64 {
		return 0, fmt.Errorf("%w：不得低于 %d", ErrBadValue, minValue.Int64)
	}
	if maxValue.Valid && n > maxValue.Int64 {
		return 0, fmt.Errorf("%w：不得高于 %d", ErrBadValue, maxValue.Int64)
	}
	return n, nil
}

func parseIntValue(raw string) (int, error) {
	trimmed := strings.TrimSpace(raw)
	var n int
	if err := json.Unmarshal([]byte(trimmed), &n); err == nil {
		return n, nil
	}
	parsed, err := strconv.Atoi(strings.Trim(trimmed, `"`))
	if err != nil {
		return 0, ErrBadValue
	}
	return parsed, nil
}

// KnownKey 只认数据字典登记过的业务参数，避免把任意键写进配置表。
func KnownKey(key string) bool {
	switch key {
	case KeyStatsDisplayThreshold, KeyCommentDisplayThreshold, KeyOfflineValidDays:
		return true
	default:
		return false
	}
}

// SecretLike 双保险：即使误加配置项，也不允许出现密钥味儿。
func SecretLike(key string) bool {
	lower := strings.ToLower(key)
	for _, token := range []string{"secret", "token", "password", "private", "key_", "apikey", "api_key"} {
		if strings.Contains(lower, token) {
			return true
		}
	}
	return false
}

func envValue(key string) (string, bool) {
	name := "CWTC_" + strings.ToUpper(strings.NewReplacer(".", "_", "-", "_").Replace(key))
	value, ok := os.LookupEnv(name)
	return value, ok && strings.TrimSpace(value) != ""
}

func nullableInt64(v sql.NullInt64) *int64 {
	if !v.Valid {
		return nil
	}
	n := v.Int64
	return &n
}
