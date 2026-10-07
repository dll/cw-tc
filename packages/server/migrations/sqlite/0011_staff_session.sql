-- 内部人员会话（管理员/教师登录，PRD §7.10 · 配套 staff 表）
CREATE TABLE IF NOT EXISTS staff_session (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    staff_id INTEGER NOT NULL REFERENCES staff(id),
    token_hash BLOB NOT NULL,
    issued_at TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    revoked_at TEXT
);
CREATE INDEX IF NOT EXISTS idx_staff_session_token ON staff_session(token_hash);
CREATE INDEX IF NOT EXISTS idx_staff_session_staff ON staff_session(staff_id);
