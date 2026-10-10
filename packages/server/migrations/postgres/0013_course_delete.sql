-- 课程软删除（教师删除草稿/驳回课程，不物理删除，PRD §7.9 · 铁律 12）
-- deleted_at 非空即视为已删除；删除留痕到 course_delete_log（含课程码/名称快照）。
CREATE TABLE IF NOT EXISTS course_delete_log (
    id BIGSERIAL PRIMARY KEY,
    course_code TEXT NOT NULL,
    course_name TEXT NOT NULL,
    deleted_by BIGINT NOT NULL REFERENCES staff(id),
    deleted_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_course_delete_log_code ON course_delete_log(course_code);
ALTER TABLE course ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
