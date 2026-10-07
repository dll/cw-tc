-- 课程审核记录（PRD §7.9 J-2 · 数据字典 §7.6）
CREATE TABLE IF NOT EXISTS course_review (
    id BIGSERIAL PRIMARY KEY,
    course_code TEXT NOT NULL REFERENCES course(course_code),
    round INTEGER NOT NULL,
    content_version INTEGER NOT NULL,
    reviewer_id BIGINT NOT NULL REFERENCES staff(id),
    reviewer_role TEXT NOT NULL,
    score_material NUMERIC(4,1) NOT NULL,
    score_objective NUMERIC(4,1) NOT NULL,
    score_case NUMERIC(4,1) NOT NULL,
    score_lesson_plan NUMERIC(4,1) NOT NULL,
    score_slides NUMERIC(4,1) NOT NULL,
    score_lab NUMERIC(4,1) NOT NULL,
    total_score NUMERIC(5,1) GENERATED ALWAYS AS (score_material + score_objective + score_case + score_lesson_plan + score_slides + score_lab) STORED,
    dimension_notes JSONB,
    issue_list JSONB,
    verdict TEXT NOT NULL,
    reviewed_at TIMESTAMPTZ NOT NULL,
    CHECK (reviewer_role IN ('admin_ops','admin_super')),
    CHECK (verdict IN ('approved','rejected')),
    CHECK (verdict <> 'rejected' OR (issue_list IS NOT NULL AND issue_list <> '[]'::jsonb))
);
CREATE INDEX IF NOT EXISTS idx_course_review_code ON course_review(course_code, round DESC);
CREATE INDEX IF NOT EXISTS idx_course_review_verdict ON course_review(verdict, reviewed_at DESC);
