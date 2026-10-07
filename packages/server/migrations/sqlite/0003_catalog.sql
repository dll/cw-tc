-- 0003_catalog（SQLite · 仅供开发与测试 方言）
-- 第三课堂 · 课程与内容（数据字典 §3）
-- ⚠️ 本文件由数据字典重建（v2.25）：表与列**逐字段对齐字典 §2~§5**。
-- ⚠️ 字典是**终态**；**本批次只建基列**，后续迁移新增的列见每个表下的 `@deferred` 行
--    （`tools/audit_prd_closure.py` 会校验：**基列 ∪ deferred == 字典列**，不多不少）。
-- ⚠️ 可直接重建的理由：**这些迁移尚未在任何持久环境执行过**（生产未部署；dev/test 每次从空库重建）。

CREATE TABLE IF NOT EXISTS major (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    sort INT NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'enabled',
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (status IN ('enabled','disabled'))
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_major_name ON major(name);

CREATE TABLE IF NOT EXISTS course (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    course_code TEXT NOT NULL,
    source TEXT NOT NULL,
    name TEXT NOT NULL,
    intro TEXT,
    line TEXT NOT NULL,
    lecture_count INT NOT NULL DEFAULT 0,
    dir TEXT,
    major_id INTEGER REFERENCES major(id),
    price_cents INTEGER NOT NULL DEFAULT 0,
    original_price_cents INTEGER,
    status TEXT NOT NULL DEFAULT 'proposed',
    sort_weight INT NOT NULL DEFAULT 0,
    free_preview_lectures INT NOT NULL DEFAULT 0,
    cert_names TEXT,
    cert_pass_lines TEXT,
    teacher_id INTEGER REFERENCES staff(id),
    ai_enabled INTEGER NOT NULL DEFAULT true,
    ai_daily_quota INT NOT NULL DEFAULT 20,
    kb_status TEXT NOT NULL DEFAULT 'none',
    submitted_at TEXT,
    reviewed_at TEXT,
    reviewed_by INTEGER REFERENCES staff(id),
    reject_reason TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (source IN ('upstream','platform')),
    CHECK (line IN ('main','aux')),
    CHECK (status IN ('proposed','building','submitted','rejected','approved','published','unpublished')),
    CHECK (kb_status IN ('none','building','ready','failed')),
    CHECK (price_cents >= 0),
    CHECK (source = 'upstream' OR teacher_id IS NOT NULL)
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_course_code ON course(course_code);
CREATE INDEX IF NOT EXISTS idx_course_status ON course(status, sort_weight DESC);
CREATE INDEX IF NOT EXISTS idx_course_major ON course(major_id);
CREATE INDEX IF NOT EXISTS idx_course_teacher ON course(teacher_id);
-- @deferred course: ai_level@0011 content_version@0011 feature_tag@0011 promote_home@0011 promote_top@0011 published_version@0011 quality_score@0011
-- @note course: 0011：`course.status` 由五态扩为七态（本文件已一次给全，避免发明中间态）

CREATE TABLE IF NOT EXISTS lecture (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    course_id INTEGER NOT NULL REFERENCES course(id),
    no INT NOT NULL,
    title TEXT NOT NULL,
    is_preview INTEGER NOT NULL DEFAULT false,
    origin TEXT NOT NULL DEFAULT 'upstream',
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_lecture_course_no ON lecture(course_id, no);
-- @deferred lecture: in_exam_scope@0015 kind@0011
-- @note lecture: 0015：补 `CHECK (origin <> 'upstream' OR in_exam_scope = true)`（该约束依赖 `in_exam_scope`，必须同批加）
-- @note lecture: 0015：补 `INDEX(course_id, origin)`

CREATE TABLE IF NOT EXISTS asset (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    lecture_id INTEGER NOT NULL REFERENCES lecture(id),
    kind TEXT NOT NULL,
    title TEXT NOT NULL,
    storage_key TEXT NOT NULL,
    size_bytes INTEGER,
    sha256 TEXT,
    sort INT NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_asset_storage_key ON asset(storage_key);
CREATE INDEX IF NOT EXISTS idx_asset_lecture ON asset(lecture_id, sort);
-- @deferred asset: aigc_label@0018 aigc_meta_written@0018

CREATE TABLE IF NOT EXISTS question (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    course_id INTEGER NOT NULL REFERENCES course(id),
    qno TEXT NOT NULL,
    level TEXT NOT NULL,
    type TEXT NOT NULL,
    gradability TEXT NOT NULL,
    stem TEXT NOT NULL,
    options TEXT,
    answer TEXT,
    analysis TEXT,
    rubric TEXT,
    score NUMERIC NOT NULL,
    evidence TEXT,
    is_override INTEGER NOT NULL DEFAULT false,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (level IN ('L1','L2','L3')),
    CHECK (gradability IN ('auto','semi_auto','manual'))
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_question_course_qno ON question(course_id, qno);
CREATE INDEX IF NOT EXISTS idx_question_course_level ON question(course_id, level, type);
