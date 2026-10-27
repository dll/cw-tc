-- 新账号默认待运营审核；审核时间仅由管理员审批动作写入。
ALTER TABLE student ADD COLUMN registration_reviewed_at TEXT;
ALTER TABLE student ADD COLUMN registration_reviewed_by INTEGER REFERENCES staff(id);
ALTER TABLE student ADD COLUMN registration_review_status TEXT NOT NULL DEFAULT 'approved';
CREATE INDEX IF NOT EXISTS idx_student_registration_review ON student(registration_reviewed_at, created_at DESC);
