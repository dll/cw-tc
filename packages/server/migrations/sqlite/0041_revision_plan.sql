-- AI 周报教师结论与改版计划（PRD §7.9 J-10）
ALTER TABLE course_insight ADD COLUMN verdict TEXT NOT NULL DEFAULT 'pending';
ALTER TABLE course_insight ADD COLUMN verdict_by INTEGER REFERENCES staff(id);
ALTER TABLE course_insight ADD COLUMN verdict_at TEXT;
ALTER TABLE course_insight ADD COLUMN verdict_note TEXT;

CREATE TABLE IF NOT EXISTS revision_plan (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    course_code TEXT NOT NULL REFERENCES course(course_code),
    source TEXT NOT NULL,
    insight_id INTEGER REFERENCES course_insight(id),
    problem TEXT NOT NULL,
    evidence TEXT NOT NULL,
    measure TEXT NOT NULL,
    scope TEXT,
    priority TEXT,
    target_metric TEXT NOT NULL,
    owner_id INTEGER REFERENCES staff(id),
    due_at TEXT,
    status TEXT NOT NULL DEFAULT 'evaluating',
    abandon_reason TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (source IN ('ai','manual','student_feedback','admin')),
    CHECK (status IN ('evaluating','accepted','in_progress','done','abandoned')),
    CHECK (priority IS NULL OR priority IN ('P0','P1','P2')),
    CHECK (evidence <> '' AND target_metric <> ''),
    CHECK (status <> 'abandoned' OR (abandon_reason IS NOT NULL AND abandon_reason <> ''))
);
CREATE INDEX IF NOT EXISTS idx_revision_plan_course ON revision_plan(course_code, status, updated_at DESC);