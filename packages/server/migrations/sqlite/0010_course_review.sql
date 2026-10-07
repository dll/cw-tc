-- 课程审核记录（PRD §7.9 J-2 · 数据字典 §7.6）
CREATE TABLE IF NOT EXISTS course_review (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    course_code TEXT NOT NULL REFERENCES course(course_code),
    round INTEGER NOT NULL,
    content_version INTEGER NOT NULL,
    reviewer_id INTEGER NOT NULL REFERENCES staff(id),
    reviewer_role TEXT NOT NULL,
    score_material REAL NOT NULL,
    score_objective REAL NOT NULL,
    score_case REAL NOT NULL,
    score_lesson_plan REAL NOT NULL,
    score_slides REAL NOT NULL,
    score_lab REAL NOT NULL,
    total_score REAL GENERATED ALWAYS AS (score_material + score_objective + score_case + score_lesson_plan + score_slides + score_lab) STORED,
    dimension_notes TEXT,
    issue_list TEXT,
    verdict TEXT NOT NULL,
    reviewed_at TEXT NOT NULL,
    CHECK (reviewer_role IN ('admin_ops','admin_super')),
    CHECK (verdict IN ('approved','rejected')),
    CHECK (verdict <> 'rejected' OR (issue_list IS NOT NULL AND issue_list <> '[]'))
);
CREATE INDEX IF NOT EXISTS idx_course_review_code ON course_review(course_code, round DESC);
CREATE INDEX IF NOT EXISTS idx_course_review_verdict ON course_review(verdict, reviewed_at DESC);
