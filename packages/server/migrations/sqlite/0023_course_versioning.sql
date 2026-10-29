-- 课程版本化：content_version（编辑/送审版本）与 published_version（线上版本）。
ALTER TABLE course ADD COLUMN content_version INTEGER NOT NULL DEFAULT 1;
ALTER TABLE course ADD COLUMN published_version INTEGER;
UPDATE course SET published_version = content_version WHERE status IN ('published','unpublished') AND published_version IS NULL;

CREATE TABLE IF NOT EXISTS course_revision (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    course_code TEXT NOT NULL REFERENCES course(course_code),
    revision_no INTEGER NOT NULL,
    content_version INTEGER NOT NULL,
    plan_id INTEGER,
    changed_digest TEXT NOT NULL DEFAULT '',
    published_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(course_code, revision_no)
);
CREATE INDEX IF NOT EXISTS idx_course_revision_code ON course_revision(course_code, revision_no DESC);
