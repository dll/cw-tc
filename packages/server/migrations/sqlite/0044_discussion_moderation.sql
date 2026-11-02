-- 讨论治理（PRD §7.15 N-5 / §9.10）：只逻辑下架，不物理删除
ALTER TABLE lecture_topic ADD COLUMN status TEXT NOT NULL DEFAULT 'visible';
ALTER TABLE lecture_topic ADD COLUMN remove_reason_kind TEXT;
ALTER TABLE lecture_reply ADD COLUMN status TEXT NOT NULL DEFAULT 'visible';
ALTER TABLE lecture_reply ADD COLUMN remove_reason_kind TEXT;
ALTER TABLE lecture_reply_staff ADD COLUMN status TEXT NOT NULL DEFAULT 'visible';
ALTER TABLE lecture_reply_staff ADD COLUMN remove_reason_kind TEXT;
CREATE INDEX IF NOT EXISTS idx_lecture_topic_status ON lecture_topic(status, created_at DESC);