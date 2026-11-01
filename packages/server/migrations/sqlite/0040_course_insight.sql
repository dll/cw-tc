-- AI 运营报告（PRD §7.9 J-10 / 数据字典 §6.6）
CREATE TABLE IF NOT EXISTS course_insight (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    course_code TEXT NOT NULL REFERENCES course(course_code),
    period_start TEXT NOT NULL,
    period_end TEXT NOT NULL,
    summary TEXT,
    signals TEXT NOT NULL DEFAULT '[]',
    feedback_clusters TEXT NOT NULL DEFAULT '[]',
    suggestions TEXT NOT NULL DEFAULT '[]',
    unknowns TEXT NOT NULL DEFAULT '[]',
    evidence_map TEXT NOT NULL DEFAULT '{}',
    model_info TEXT,
    tokens INTEGER NOT NULL DEFAULT 0,
    generated_at TEXT NOT NULL,
    CHECK (evidence_map <> '')
);
CREATE INDEX IF NOT EXISTS idx_course_insight_course ON course_insight(course_code, generated_at DESC);