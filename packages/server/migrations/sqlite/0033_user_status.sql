-- 学员账号状态（PRD §7.10.10（三）· 数据字典 §7.1）
-- 封禁必须可追溯：原因类别 + 说明 + 期限；永久封禁仅 super。
ALTER TABLE student ADD COLUMN status_reason_kind TEXT
    CHECK (status_reason_kind IS NULL OR status_reason_kind IN
      ('sms_abuse','payment_abuse','identity_mismatch','exam_fraud','serious_violation','login_fail'));
ALTER TABLE student ADD COLUMN status_note TEXT;
ALTER TABLE student ADD COLUMN status_until TEXT;
ALTER TABLE student ADD COLUMN status_by INTEGER REFERENCES staff(id);
CREATE INDEX IF NOT EXISTS idx_student_status ON student(status, status_until);
