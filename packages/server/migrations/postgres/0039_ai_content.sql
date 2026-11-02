-- AI 生成内容与双审（PRD §7.16 / 数据字典 §6.9）
ALTER TABLE lecture ADD COLUMN IF NOT EXISTS in_exam_scope BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE asset ADD COLUMN IF NOT EXISTS aigc_label TEXT;
ALTER TABLE asset ADD COLUMN IF NOT EXISTS aigc_meta_written BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE IF NOT EXISTS ai_content_draft (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    course_code TEXT NOT NULL REFERENCES course(course_code),
    lecture_no INTEGER,
    draft_scope TEXT NOT NULL DEFAULT 'single',
    lectures JSONB NOT NULL DEFAULT '[]'::jsonb,
    lecture_count INTEGER NOT NULL DEFAULT 1,
    source_citations JSONB NOT NULL DEFAULT '[]'::jsonb,
    auto_check JSONB,
    auto_check_passed BOOLEAN NOT NULL DEFAULT FALSE,
    model_id TEXT NOT NULL,
    model_version TEXT NOT NULL,
    filing_no TEXT NOT NULL,
    aigc_labeled BOOLEAN NOT NULL DEFAULT FALSE,
    aigc_meta_written BOOLEAN NOT NULL DEFAULT FALSE,
    kind TEXT NOT NULL,
    trigger_kind TEXT NOT NULL,
    insight_id BIGINT,
    plan_id BIGINT,
    trigger_topic_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    content_md TEXT NOT NULL,
    model_info JSONB,
    tokens INTEGER DEFAULT 0,
    ai_labeled BOOLEAN NOT NULL DEFAULT TRUE,
    status TEXT NOT NULL DEFAULT 'drafting',
    reject_count INTEGER NOT NULL DEFAULT 0,
    content_version INTEGER,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (draft_scope IN ('chapter','single')),
    CHECK (kind IN ('new_lecture','explain_supplement','example','lab_guide','errata')),
    CHECK (trigger_kind IN ('insight','teacher_mark','metric')),
    CHECK (status IN ('drafting','pending_teacher','teacher_approved','teacher_rejected','pending_admin','admin_approved','admin_rejected','published','abandoned')),
    CHECK (ai_labeled = TRUE)
);
CREATE INDEX IF NOT EXISTS idx_ai_draft_status ON ai_content_draft(status, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_ai_draft_course ON ai_content_draft(course_code, created_at DESC);

CREATE TABLE IF NOT EXISTS ai_content_review (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    draft_id BIGINT NOT NULL REFERENCES ai_content_draft(id) ON DELETE CASCADE,
    stage TEXT NOT NULL,
    reviewer_id BIGINT NOT NULL REFERENCES staff(id),
    verdict TEXT NOT NULL,
    issue_list JSONB,
    edited_by_teacher BOOLEAN NOT NULL DEFAULT FALSE,
    reviewed_content_hash TEXT NOT NULL,
    reviewed_at TIMESTAMPTZ NOT NULL,
    CHECK (stage IN ('teacher','admin')),
    CHECK (verdict IN ('approved','rejected')),
    CHECK (verdict <> 'rejected' OR (issue_list IS NOT NULL AND issue_list <> '[]')),
    UNIQUE(draft_id, stage, reviewed_content_hash)
);