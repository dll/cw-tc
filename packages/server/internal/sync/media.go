package sync

import (
	"context"
	"database/sql"
)

// MediaEntry 是一行待写入 asset 的视频/音频清单（由 mediasync 本地扫描+上传 COS 后产出）。
type MediaEntry struct {
	StorageKey string `json:"storage_key"`
	Kind       string `json:"kind"` // video / audio
	Size       int64  `json:"size"`
	CourseCode string `json:"course_code"`
	LectureNo  int    `json:"lecture_no"`
}

// ImportMediaAssets 把媒体清单写入 asset 表（视频/音频走 COS，storage_key 即 COS 对象键）。
// 讲次不存在则跳过；按 storage_key upsert（幂等，重复导入不重复建行）。
func ImportMediaAssets(ctx context.Context, db *sql.DB, entries []MediaEntry) (int, error) {
	inserted := 0
	for _, e := range entries {
		var lectureID int64
		err := db.QueryRowContext(ctx,
			`SELECT l.id FROM lecture l JOIN course c ON c.id = l.course_id
			 WHERE c.course_code = $1 AND l.no = $2 AND l.origin = 'upstream'`, e.CourseCode, e.LectureNo).Scan(&lectureID)
		if err != nil {
			continue // 讲次不存在则跳过（源目录结构变动时容错）
		}
		title := "视频"
		if e.Kind == "audio" {
			title = "音频"
		}
		if _, err := db.ExecContext(ctx,
			`INSERT INTO asset(lecture_id, kind, title, storage_key, size_bytes, sort)
			 VALUES($1,$2,$3,$4,$5,0)
			 ON CONFLICT(storage_key) DO UPDATE SET lecture_id=excluded.lecture_id, kind=excluded.kind, title=excluded.title, size_bytes=excluded.size_bytes`,
			lectureID, e.Kind, title, e.StorageKey, e.Size); err != nil {
			return inserted, err
		}
		inserted++
	}
	return inserted, nil
}
