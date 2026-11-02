-- 改版计划与 AI 建议 / 版本效果回流（PRD §7.9 J-10）
ALTER TABLE revision_plan ADD COLUMN IF NOT EXISTS suggestion_index INTEGER;
CREATE INDEX IF NOT EXISTS idx_revision_plan_suggestion ON revision_plan(suggestion_index);
