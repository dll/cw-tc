-- 课程质量分与 AI 化等级（PRD §7.2 B-9 / §7.10.2）。
ALTER TABLE course ADD COLUMN quality_score NUMERIC(5,1);
ALTER TABLE course ADD COLUMN ai_level TEXT CHECK (ai_level IS NULL OR ai_level IN ('LA','LB','LC'));
CREATE INDEX IF NOT EXISTS idx_course_quality ON course(quality_score);
