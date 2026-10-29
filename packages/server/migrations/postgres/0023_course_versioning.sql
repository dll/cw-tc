-- 课程版本化：content_version（编辑/送审版本）与 published_version（线上版本）。
-- 已上架课程编辑期间两者可不同；审核与发布都以版本号为准。
ALTER TABLE course ADD COLUMN content_version INTEGER NOT NULL DEFAULT 1;
ALTER TABLE course ADD COLUMN published_version INTEGER;
-- 已上架/已下架的存量课程：把当前内容版本回填为线上版本，避免误显示“未发布”。
UPDATE course SET published_version = content_version WHERE status IN ('published','unpublished') AND published_version IS NULL;

-- 改版记录：每次发布（含重新上架）追加一条，历史不可覆盖。
CREATE TABLE IF NOT EXISTS course_revision (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    course_code TEXT NOT NULL REFERENCES course(course_code),
    revision_no INTEGER NOT NULL,
    content_version INTEGER NOT NULL,
    plan_id BIGINT,
    changed_digest TEXT NOT NULL DEFAULT '',
    published_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(course_code, revision_no)
);
CREATE INDEX IF NOT EXISTS idx_course_revision_code ON course_revision(course_code, revision_no DESC);
