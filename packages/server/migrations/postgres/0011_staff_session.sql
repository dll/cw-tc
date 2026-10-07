-- 内部人员会话（管理员/教师登录，PRD §7.10 · 配套 staff 表）
CREATE TABLE IF NOT EXISTS staff_session (
    id BIGSERIAL PRIMARY KEY,
    staff_id BIGINT NOT NULL REFERENCES staff(id),
    token_hash BYTEA NOT NULL,
    issued_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_staff_session_token ON staff_session(token_hash);
CREATE INDEX IF NOT EXISTS idx_staff_session_staff ON staff_session(staff_id);
