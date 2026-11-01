-- AI 生成内容与双审（PRD §7.16 / 数据字典 §6.9）
ALTER TABLE lecture ADD COLUMN in_exam_scope INTEGER NOT NULL DEFAULT 1;
ALTER TABLE asset ADD COLUMN aigc_label TEXT;
ALTER TABLE asset ADD COLUMN aigc_meta_written INTEGER NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS ai_content_draft (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    course_code TEXT NOT NULL REFERENCES course(course_code),
    lecture_no INTEGER,
    draft_scope TEXT NOT NULL DEFAULT 'single',
    lectures TEXT NOT NULL DEFAULT '[]',
    lecture_count INTEGER NOT NULL DEFAULT 1,
    source_citations TEXT NOT NULL DEFAULT '[]',
    auto_check TEXT,
    auto_check_passed INTEGER NOT NULL DEFAULT 0,
    model_id TEXT NOT NULL,
    model_version TEXT NOT NULL,
    filing_no TEXT NOT NULL,
    aigc_labeled INTEGER NOT NULL DEFAULT false,
    aigc_meta_written INTEGER NOT NULL DEFAULT false,
    kind TEXT NOT NULL,
    trigger_kind TEXT NOT NULL,
    insight_id INTEGER,
    plan_id INTEGER,
    trigger_topic_ids TEXT NOT NULL DEFAULT '[]',
    content_md TEXT NOT NULL,
    model_info TEXT,
    tokens INTEGER DEFAULT 0,
    ai_labeled INTEGER NOT NULL DEFAULT true,
    status TEXT NOT NULL DEFAULT 'drafting',
    reject_count INTEGER NOT NULL DEFAULT 0,
    content_version INTEGER,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (draft_scope IN ('chapter','single')),
    CHECK (kind IN ('new_lecture','explain_supplement','example','lab_guide','errata')),
    CHECK (trigger_kind IN ('insight','teacher_mark','metric')),
    CHECK (status IN ('drafting','pending_teacher','teacher_approved','teacher_rejected','pending_admin','admin_approved','admin_rejected','published','abandoned')),
    CHECK (ai_labeled = true)
);
CREATE INDEX IF NOT EXISTS idx_ai_draft_status ON ai_content_draft(status, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_ai_draft_course ON ai_content_draft(course_code, created_at DESC);

CREATE TABLE IF NOT EXISTS ai_content_review (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    draft_id INTEGER NOT NULL REFERENCES ai_content_draft(id) ON DELETE CASCADE,
    stage TEXT NOT NULL,
    reviewer_id INTEGER NOT NULL REFERENCES staff(id),
    verdict TEXT NOT NULL,
    issue_list TEXT,
    edited_by_teacher INTEGER NOT NULL DEFAULT false,
    reviewed_content_hash TEXT NOT NULL,
    reviewed_at TEXT NOT NULL,
    CHECK (stage IN ('teacher','admin')),
    CHECK (verdict IN ('approved','rejected')),
    CHECK (verdict <> 'rejected' OR (issue_list IS NOT NULL AND issue_list <> '[]')),
    UNIQUE(draft_id, stage, reviewed_content_hash)
);