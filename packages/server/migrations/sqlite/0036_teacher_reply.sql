-- 教师讨论回复（PRD §7.15 N-5 / §7.9 J-6）
-- 学员回复在 lecture_reply；教师回复单独落表，避免把教师写成 student_id。
-- 公开列表合并两表，教师回复带官方标识；教师不能删除学员帖 / 回复。
CREATE TABLE IF NOT EXISTS lecture_reply_staff (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    topic_id INTEGER NOT NULL REFERENCES lecture_topic(id) ON DELETE CASCADE,
    staff_id INTEGER NOT NULL REFERENCES staff(id),
    body TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_lecture_reply_staff_topic ON lecture_reply_staff(topic_id, created_at, id);
