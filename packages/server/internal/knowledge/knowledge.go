// Package knowledge 构建课程知识库切片并提供带出处的检索（PRD §7.5 H-2）。
//
// 当前实现使用 SQLite/PostgreSQL 通用文本表 + 关键词打分（开发/测试可用、无外部依赖）；
// 生产可在此表上追加 pgvector 列，不改业务调用方与引用契约。
package knowledge

import (
	"context"
	"database/sql"
	"errors"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	maxAssetBytes = 5 << 20
	chunkRunes    = 900
	maxChunks     = 5000
)

var (
	ErrInput         = errors.New("知识库参数不合法")
	ErrCourseMissing = errors.New("课程不存在")
)

var htmlTagRE = regexp.MustCompile(`<[^>]+>`)

type Stats struct {
	CourseCode string `json:"course_code"`
	Assets     int    `json:"assets"`
	Chunks     int    `json:"chunks"`
	Chars      int    `json:"chars"`
	BuiltAt    string `json:"built_at"`
}

type Status struct {
	CourseCode string  `json:"course_code"`
	CourseName string  `json:"course_name"`
	KbStatus   string  `json:"kb_status"`
	Chunks     int     `json:"chunks"`
	Assets     int     `json:"assets"`
	LastBuilt  *string `json:"last_built"`
}

type Match struct {
	LectureNo    int    `json:"lecture_no"`
	LectureTitle string `json:"lecture_title"`
	AssetID      int64  `json:"asset_id"`
	AssetTitle   string `json:"asset_title"`
	StorageKey   string `json:"storage_key"`
	ChunkIndex   int    `json:"chunk_index"`
	Content      string `json:"content"`
	Score        int    `json:"score"`
}

// BuildCourse 重建某课知识库：只读课程资源，切片后整课替换（幂等）。
func BuildCourse(ctx context.Context, db *sql.DB, contentDir, courseCode string) (Stats, error) {
	if db == nil || strings.TrimSpace(contentDir) == "" || courseCode == "" {
		return Stats{}, ErrInput
	}
	var courseID int64
	if err := db.QueryRowContext(ctx, `SELECT id FROM course WHERE course_code = $1 AND deleted_at IS NULL`, courseCode).Scan(&courseID); err != nil {
		if err == sql.ErrNoRows {
			return Stats{}, ErrCourseMissing
		}
		return Stats{}, err
	}
	_, _ = db.ExecContext(ctx, `UPDATE course SET kb_status = 'building' WHERE id = $1`, courseID)
	stats, err := collect(ctx, db, contentDir, courseCode, courseID)
	if err != nil {
		_, _ = db.ExecContext(ctx, `UPDATE course SET kb_status = 'failed' WHERE id = $1`, courseID)
		return Stats{}, err
	}
	_, _ = db.ExecContext(ctx, `UPDATE course SET kb_status = 'ready' WHERE id = $1`, courseID)
	return stats, nil
}

func collect(ctx context.Context, db *sql.DB, contentDir, courseCode string, courseID int64) (Stats, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT l.no, l.title, a.id, a.title, a.storage_key, a.kind
		 FROM asset a JOIN lecture l ON l.id = a.lecture_id
		 WHERE l.course_id = $1 AND a.kind IN ('markdown','html','code','frozen_output')
		 ORDER BY l.no, a.sort, a.id`, courseID)
	if err != nil {
		return Stats{}, err
	}
	defer rows.Close()
	type piece struct {
		lectureNo    int
		lectureTitle string
		assetID      int64
		assetTitle   string
		storageKey   string
		index        int
		content      string
	}
	pieces := make([]piece, 0)
	assets := 0
	root := filepath.Clean(contentDir) + string(os.PathSeparator)
	for rows.Next() {
		var no, index int
		var lectureTitle, assetTitle, key, kind string
		var assetID int64
		if err := rows.Scan(&no, &lectureTitle, &assetID, &assetTitle, &key, &kind); err != nil {
			return Stats{}, err
		}
		path := filepath.Join(contentDir, filepath.FromSlash(key))
		if !strings.HasPrefix(filepath.Clean(path), root) {
			continue
		}
		info, err := os.Stat(path)
		if err != nil || info.Size() == 0 || info.Size() > maxAssetBytes {
			continue
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		text := string(raw)
		if kind == "html" {
			text = html.UnescapeString(htmlTagRE.ReplaceAllString(text, " "))
		}
		parts := splitText(text, chunkRunes)
		if len(parts) == 0 {
			continue
		}
		assets++
		for _, part := range parts {
			pieces = append(pieces, piece{
				lectureNo: no, lectureTitle: lectureTitle, assetID: assetID,
				assetTitle: assetTitle, storageKey: key, index: index, content: part,
			})
			index++
		}
	}
	if err := rows.Err(); err != nil {
		return Stats{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Stats{}, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM knowledge_chunk WHERE course_code = $1`, courseCode); err != nil {
		return Stats{}, err
	}
	chars := 0
	for _, item := range pieces {
		count := len([]rune(item.content))
		chars += count
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO knowledge_chunk(course_code, lecture_no, lecture_title, asset_id, asset_title,
			   storage_key, chunk_index, content, char_count, created_at)
			 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
			courseCode, item.lectureNo, item.lectureTitle, item.assetID, item.assetTitle,
			item.storageKey, item.index, item.content, count, now); err != nil {
			return Stats{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Stats{}, err
	}
	return Stats{CourseCode: courseCode, Assets: assets, Chunks: len(pieces), Chars: chars, BuiltAt: now}, nil
}

// Search 按关键词与当前讲次偏好检索切片；返回内容与出处（讲次 / 资源 / 存储键）。
func Search(ctx context.Context, db *sql.DB, courseCode string, lectureNo int, tokens []string, limit int) ([]Match, error) {
	if db == nil || courseCode == "" {
		return nil, ErrInput
	}
	if limit <= 0 || limit > 20 {
		limit = 8
	}
	rows, err := db.QueryContext(ctx,
		`SELECT lecture_no, lecture_title, asset_id, asset_title, storage_key, chunk_index, content
		 FROM knowledge_chunk WHERE course_code = $1 ORDER BY lecture_no, chunk_index LIMIT $2`,
		courseCode, maxChunks)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	matches := make([]Match, 0)
	lowerTokens := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if token = strings.ToLower(strings.TrimSpace(token)); token != "" {
			lowerTokens = append(lowerTokens, token)
		}
	}
	for rows.Next() {
		var item Match
		if err := rows.Scan(&item.LectureNo, &item.LectureTitle, &item.AssetID,
			&item.AssetTitle, &item.StorageKey, &item.ChunkIndex, &item.Content); err != nil {
			return nil, err
		}
		score := 0
		tokenHits := 0
		if item.LectureNo == lectureNo {
			score += 5
		}
		lower := strings.ToLower(item.Content)
		for _, token := range lowerTokens {
			if strings.Contains(lower, token) {
				score += 2
				tokenHits++
			}
		}
		if score <= 0 || (len(lowerTokens) > 0 && tokenHits == 0) {
			continue
		}
		item.Score = score
		matches = append(matches, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].Score != matches[j].Score {
			return matches[i].Score > matches[j].Score
		}
		if matches[i].LectureNo != matches[j].LectureNo {
			return matches[i].LectureNo < matches[j].LectureNo
		}
		return matches[i].ChunkIndex < matches[j].ChunkIndex
	})
	if len(matches) > limit {
		matches = matches[:limit]
	}
	return matches, nil
}

// ListStatus 返回课程知识库状态（切片数 / 资源数 / 最后构建时间）。
func ListStatus(ctx context.Context, db *sql.DB, courseCode string) ([]Status, error) {
	if db == nil {
		return nil, ErrInput
	}
	rows, err := db.QueryContext(ctx,
		`SELECT c.course_code, c.name, c.kb_status,
		        (SELECT COUNT(*) FROM knowledge_chunk k WHERE k.course_code = c.course_code),
		        (SELECT COUNT(DISTINCT k.asset_id) FROM knowledge_chunk k WHERE k.course_code = c.course_code),
		        (SELECT MAX(created_at) FROM knowledge_chunk k WHERE k.course_code = c.course_code)
		 FROM course c
		 WHERE c.deleted_at IS NULL AND ($1 = '' OR c.course_code = $1)
		 ORDER BY c.id`, courseCode)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Status, 0)
	for rows.Next() {
		var item Status
		var last sql.NullString
		if err := rows.Scan(&item.CourseCode, &item.CourseName, &item.KbStatus, &item.Chunks, &item.Assets, &last); err != nil {
			return nil, err
		}
		if last.Valid {
			value := last.String
			item.LastBuilt = &value
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func splitText(text string, maxRunes int) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	parts := make([]string, 0)
	for _, paragraph := range strings.Split(text, "\n\n") {
		paragraph = strings.TrimSpace(paragraph)
		if paragraph == "" {
			continue
		}
		runes := []rune(paragraph)
		for len(runes) > maxRunes {
			parts = append(parts, strings.TrimSpace(string(runes[:maxRunes])))
			runes = runes[maxRunes:]
		}
		if len(runes) > 0 {
			parts = append(parts, strings.TrimSpace(string(runes)))
		}
	}
	return parts
}
