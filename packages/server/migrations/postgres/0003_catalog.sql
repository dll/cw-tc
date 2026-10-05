-- 0003_catalog.sql
-- 第三课堂 · 课程与内容迁移（数据字典 §3）
CREATE TABLE IF NOT EXISTS major (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    sort INT NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'enabled',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS course (
    id BIGSERIAL PRIMARY KEY,
    course_code TEXT NOT NULL UNIQUE,
    source TEXT NOT NULL CHECK (source IN ('upstream','platform')),
    name TEXT NOT NULL,
    line TEXT NOT NULL CHECK (line IN ('main','aux')),
    lecture_count INT NOT NULL DEFAULT 0,
    major_id BIGINT REFERENCES major(id),
    price_cents BIGINT NOT NULL DEFAULT 0 CHECK (price_cents >= 0),
    status TEXT NOT NULL DEFAULT 'proposed',
    teacher_id BIGINT REFERENCES staff(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (source = 'upstream' OR teacher_id IS NOT NULL)
);
CREATE TABLE IF NOT EXISTS lecture (
    id BIGSERIAL PRIMARY KEY,
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
    id BIGSERIAL PRIMARY KEY,
    lecture_id BIGINT NOT NULL REFERENCES lecture(id),
    kind TEXT NOT NULL,
    title TEXT NOT NULL,
    storage_key TEXT NOT NULL UNIQUE,
    sort INT NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS question (
    id BIGSERIAL PRIMARY KEY,
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
