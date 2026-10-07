-- 风控黑名单（§9.13）：粒度限 手机号 / 设备 / 支付账户；必须有期限、可申诉。
CREATE TABLE IF NOT EXISTS risk_blacklist (
    id BIGSERIAL PRIMARY KEY,
    target_type TEXT NOT NULL CHECK (target_type IN ('phone', 'device', 'payment_account')),
    target_value TEXT NOT NULL,
    reason TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'expired', 'appealed')),
    expires_at TIMESTAMPTZ NOT NULL,
    created_by BIGINT REFERENCES staff(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    appeal_reason TEXT,
    appealed_at TIMESTAMPTZ
);

-- 同粒度同值同一时刻只允许一条生效（部分唯一索引）。
CREATE UNIQUE INDEX IF NOT EXISTS uq_risk_blacklist_active
    ON risk_blacklist(target_type, target_value) WHERE status = 'active';
