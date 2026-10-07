-- 0006_promo（SQLite · 仅供开发与测试 方言）
-- 第三课堂 · 优惠码（数据字典 §6.11）

CREATE TABLE IF NOT EXISTS promo_code (
    code TEXT PRIMARY KEY,
    kind TEXT NOT NULL,
    value INTEGER NOT NULL,
    total_limit INTEGER,
    per_student_limit INTEGER NOT NULL DEFAULT 1,
    valid_from TEXT NOT NULL,
    valid_to TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft',
    created_by INTEGER,
    CHECK (kind IN ('amount_off','percent_off')),
    CHECK (status IN ('draft','active','exhausted','expired','disabled'))
);

CREATE TABLE IF NOT EXISTS promo_redemption (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    promo_code TEXT NOT NULL,
    student_id INTEGER NOT NULL,
    order_no TEXT NOT NULL,
    discount_cents INTEGER NOT NULL,
    at TEXT NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_promo_redemption ON promo_redemption(promo_code, student_id, order_no);
