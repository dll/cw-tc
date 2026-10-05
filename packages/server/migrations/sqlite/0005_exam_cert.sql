-- 0005_exam_cert.sql（**SQLite 方言 · 仅供开发与测试**）
-- 与 `migrations/postgres/0005_exam_cert.sql` 的表与列**一一对应**；差异仅类型与自增写法。
-- ⚠️ 生产不使用本文件；上线走 postgres/ 那一套（见 migrations/README.md）。
-- 本文件由 `migrations/postgres/0005_exam_cert.sql` 机械翻译：BIGSERIAL PRIMARY KEY×5 · TIMESTAMPTZ×7 · DEFAULT now()×3 · JSONB NOT NULL DEFAULT '{}'::jsonb×1
CREATE TABLE IF NOT EXISTS exam_session (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    student_id BIGINT NOT NULL REFERENCES student(id),
    course_id BIGINT NOT NULL REFERENCES course(id),
    level TEXT NOT NULL CHECK (level IN ('L1','L2','L3')),
    answers TEXT NOT NULL DEFAULT '{}',
    status TEXT NOT NULL DEFAULT 'ongoing' CHECK (status IN ('ongoing','submitted','graded')),
    started_at TEXT NOT NULL,
    submitted_at TEXT,
    server_deadline_at TEXT NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_exam_ongoing ON exam_session(student_id, course_id, level) WHERE status = 'ongoing';
CREATE TABLE IF NOT EXISTS exam_result (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    exam_session_id BIGINT NOT NULL REFERENCES exam_session(id),
    score NUMERIC(6,1) NOT NULL,
    passed BOOLEAN NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS manual_review (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    exam_session_id BIGINT NOT NULL REFERENCES exam_session(id),
    reviewer_id BIGINT REFERENCES staff(id),
    verdict TEXT NOT NULL,
    reason TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS cert_no_sequence (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    next_value BIGINT NOT NULL DEFAULT 1
);
CREATE TABLE IF NOT EXISTS certificate (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    certificate_no TEXT NOT NULL UNIQUE,
    student_id BIGINT NOT NULL REFERENCES student(id),
    course_id BIGINT NOT NULL REFERENCES course(id),
    level TEXT NOT NULL CHECK (level IN ('L1','L2','L3')),
    issued_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    revoked_at TEXT
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_certificate_active ON certificate(student_id, course_id, level) WHERE revoked_at IS NULL;
