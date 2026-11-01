-- AI 运营报告（PRD §7.9 J-10 / 数据字典 §6.6）
CREATE TABLE IF NOT EXISTS course_insight (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    course_code TEXT NOT NULL REFERENCES course(course_code),
    period_start DATE NOT NULL,
    period_end DATE NOT NULL,
    summary TEXT,
    signals JSONB NOT NULL DEFAULT '[]'::jsonb,
    feedback_clusters JSONB NOT NULL DEFAULT '[]'::jsonb,
    suggestions JSONB NOT NULL DEFAULT '[]'::jsonb,
    unknowns JSONB NOT NULL DEFAULT '[]'::jsonb,
    evidence_map JSONB NOT NULL DEFAULT '{}'::jsonb,
    model_info JSONB,
    tokens INTEGER NOT NULL DEFAULT 0,
    generated_at TIMESTAMPTZ NOT NULL,
    CHECK (evidence_map <> '{}'::jsonb)
);
CREATE INDEX IF NOT EXISTS idx_course_insight_course ON course_insight(course_code, generated_at DESC);