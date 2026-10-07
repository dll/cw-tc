-- 0006_promo（PostgreSQL · 生产权威方言）
-- 第三课堂 · 优惠码（数据字典 §6.11）

CREATE TABLE IF NOT EXISTS promo_code (
    code TEXT PRIMARY KEY,
    kind TEXT NOT NULL,
    value BIGINT NOT NULL,
    total_limit BIGINT,
    per_student_limit BIGINT NOT NULL DEFAULT 1,
    valid_from TIMESTAMPTZ NOT NULL,
    valid_to TIMESTAMPTZ NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft',
    created_by BIGINT,
    CHECK (kind IN ('amount_off','percent_off')),
    CHECK (status IN ('draft','active','exhausted','expired','disabled'))
);

CREATE TABLE IF NOT EXISTS promo_redemption (
    id BIGSERIAL PRIMARY KEY,
    promo_code TEXT NOT NULL,
    student_id BIGINT NOT NULL,
    order_no TEXT NOT NULL,
    discount_cents BIGINT NOT NULL,
    at TIMESTAMPTZ NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_promo_redemption ON promo_redemption(promo_code, student_id, order_no);
