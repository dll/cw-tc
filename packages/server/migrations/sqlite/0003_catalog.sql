-- 0003_catalog.sql（**SQLite 方言 · 仅供开发与测试**）
-- 与 `migrations/postgres/0003_catalog.sql` 的表与列**一一对应**；差异仅类型与自增写法。
-- ⚠️ 生产不使用本文件；上线走 postgres/ 那一套（见 migrations/README.md）。
-- 本文件由 `migrations/postgres/0003_catalog.sql` 机械翻译：BIGSERIAL PRIMARY KEY×5 · TIMESTAMPTZ×2 · DEFAULT now()×2
CREATE TABLE IF NOT EXISTS major (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    sort INT NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'enabled',
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS course (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    course_code TEXT NOT NULL UNIQUE,
    source TEXT NOT NULL CHECK (source IN ('upstream','platform')),
    name TEXT NOT NULL,
    line TEXT NOT NULL CHECK (line IN ('main','aux')),
    lecture_count INT NOT NULL DEFAULT 0,
    major_id BIGINT REFERENCES major(id),
    price_cents BIGINT NOT NULL DEFAULT 0 CHECK (price_cents >= 0),
    status TEXT NOT NULL DEFAULT 'proposed',
    teacher_id BIGINT REFERENCES staff(id),
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (source = 'upstream' OR teacher_id IS NOT NULL)
);
CREATE TABLE IF NOT EXISTS lecture (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    course_id BIGINT NOT NULL REFERENCES course(id),
    no INT NOT NULL,
    title TEXT NOT NULL,
    is_preview BOOLEAN NOT NULL DEFAULT false,
    origin TEXT NOT NULL DEFAULT 'upstream' CHECK (origin IN ('upstream','platform','ai_assisted')),
    in_exam_scope BOOLEAN NOT NULL DEFAULT true,
    UNIQUE(course_id, no),
    CHECK (origin <> 'upstream' OR in_exam_scope = true)
);
CREATE TABLE IF NOT EXISTS asset (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    lecture_id BIGINT NOT NULL REFERENCES lecture(id),
    kind TEXT NOT NULL,
    title TEXT NOT NULL,
    storage_key TEXT NOT NULL UNIQUE,
    sort INT NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS question (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    course_id BIGINT NOT NULL REFERENCES course(id),
    qno TEXT NOT NULL,
    level TEXT NOT NULL CHECK (level IN ('L1','L2','L3')),
    type TEXT NOT NULL,
    gradability TEXT NOT NULL,
    stem TEXT NOT NULL,
    answer TEXT,
    analysis TEXT,
    score NUMERIC(6,1) NOT NULL,
    is_override BOOLEAN NOT NULL DEFAULT false,
    UNIQUE(course_id, qno)
);
