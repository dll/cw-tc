-- 教师下架 / 紧急纠错申请（PRD §7.9 J-8）。
CREATE TABLE IF NOT EXISTS content_request (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    course_code TEXT NOT NULL REFERENCES course(course_code),
    teacher_id INTEGER NOT NULL REFERENCES staff(id),
    kind TEXT NOT NULL,
    reason_kind TEXT NOT NULL,
    reason_text TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending',
    handled_by INTEGER REFERENCES staff(id),
    handled_at TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (kind IN ('offline','urgent_fix')),
    CHECK (status IN ('pending','approved','rejected'))
);
CREATE INDEX IF NOT EXISTS idx_content_request_status ON content_request(status, created_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS uq_content_request_pending ON content_request(course_code, kind) WHERE status = 'pending';
