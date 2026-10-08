-- 审计日志（PRD §7.10.9 · 数据字典 §7.3）：高危操作留痕，只增不改不删
CREATE TABLE IF NOT EXISTS audit_log (
    id BIGSERIAL PRIMARY KEY,
    actor_type TEXT NOT NULL,
    actor_id BIGINT NOT NULL,
    actor_role TEXT,
    action TEXT NOT NULL,
    target_type TEXT,
    target_id TEXT,
    before_value TEXT,
    after_value TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (actor_type IN ('student','staff','system'))
);
CREATE INDEX IF NOT EXISTS idx_audit_actor ON audit_log(actor_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_action ON audit_log(action, created_at DESC);
