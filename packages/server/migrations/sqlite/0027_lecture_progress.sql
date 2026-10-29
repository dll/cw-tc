-- 每讲学习进度（§7.4 B-5 / §7.13 L-1）：以「讲」为最小单位。
CREATE TABLE IF NOT EXISTS lecture_progress (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    student_id INTEGER NOT NULL REFERENCES student(id),
    lecture_id INTEGER NOT NULL REFERENCES lecture(id) ON DELETE CASCADE,
    position_seconds INTEGER NOT NULL DEFAULT 0,
    completed_at TEXT,
    last_studied_at TEXT NOT NULL,
    UNIQUE(student_id, lecture_id)
);
CREATE INDEX IF NOT EXISTS idx_lecture_progress_student ON lecture_progress(student_id, last_studied_at DESC);
