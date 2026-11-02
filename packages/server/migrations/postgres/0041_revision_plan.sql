-- AI 周报教师结论与改版计划（PRD §7.9 J-10）
ALTER TABLE course_insight ADD COLUMN IF NOT EXISTS verdict TEXT NOT NULL DEFAULT 'pending';
ALTER TABLE course_insight ADD COLUMN IF NOT EXISTS verdict_by BIGINT REFERENCES staff(id);
ALTER TABLE course_insight ADD COLUMN IF NOT EXISTS verdict_at TIMESTAMPTZ;
ALTER TABLE course_insight ADD COLUMN IF NOT EXISTS verdict_note TEXT;

CREATE TABLE IF NOT EXISTS revision_plan (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    course_code TEXT NOT NULL REFERENCES course(course_code),
    source TEXT NOT NULL,
    insight_id BIGINT REFERENCES course_insight(id),
    problem TEXT NOT NULL,
    evidence TEXT NOT NULL,
    measure TEXT NOT NULL,
    scope JSONB,
    priority TEXT,
    target_metric TEXT NOT NULL,
    owner_id BIGINT REFERENCES staff(id),
    due_at TIMESTAMPTZ,
    status TEXT NOT NULL DEFAULT 'evaluating',
    abandon_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (source IN ('ai','manual','student_feedback','admin')),
    CHECK (status IN ('evaluating','accepted','in_progress','done','abandoned')),
    CHECK (priority IS NULL OR priority IN ('P0','P1','P2')),
    CHECK (evidence <> '' AND target_metric <> ''),
    CHECK (status <> 'abandoned' OR (abandon_reason IS NOT NULL AND abandon_reason <> ''))
);
CREATE INDEX IF NOT EXISTS idx_revision_plan_course ON revision_plan(course_code, status, updated_at DESC);