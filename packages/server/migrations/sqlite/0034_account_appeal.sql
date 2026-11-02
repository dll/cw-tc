-- 账号申诉（PRD §7.10.10（三）/ 数据字典 §7.7）
-- 登录页申诉以手机号 + 短信验证码为凭据（被封无法登录）；管理员处理结果须通知学员。
CREATE TABLE IF NOT EXISTS account_appeal (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    student_id INTEGER REFERENCES student(id),
    phone TEXT NOT NULL,
    kind TEXT NOT NULL,
    content_text TEXT NOT NULL,
    attachments TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    handled_by INTEGER REFERENCES staff(id),
    handled_at TEXT,
    result_note TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (kind IN ('ban','identity','score','refund')),
    CHECK (status IN ('pending','accepted','rejected')),
    CHECK (status = 'pending' OR result_note IS NOT NULL)
);
CREATE INDEX IF NOT EXISTS idx_account_appeal_status ON account_appeal(status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_account_appeal_phone ON account_appeal(phone, created_at DESC);
