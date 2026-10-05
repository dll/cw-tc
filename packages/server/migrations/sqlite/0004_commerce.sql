-- 0004_commerce.sql（**SQLite 方言 · 仅供开发与测试**）
-- 与 `migrations/postgres/0004_commerce.sql` 的表与列**一一对应**；差异仅类型与自增写法。
-- ⚠️ 生产不使用本文件；上线走 postgres/ 那一套（见 migrations/README.md）。
-- 本文件由 `migrations/postgres/0004_commerce.sql` 机械翻译：BIGSERIAL PRIMARY KEY×5 · TIMESTAMPTZ×6 · DEFAULT now()×5 · reconcile_date DATE×1
CREATE TABLE IF NOT EXISTS entitlement (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    student_id BIGINT NOT NULL REFERENCES student(id),
    course_id BIGINT NOT NULL REFERENCES course(id),
    status TEXT NOT NULL DEFAULT 'active',
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    revoked_at TEXT
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_entitlement_active ON entitlement(student_id, course_id) WHERE revoked_at IS NULL;
CREATE TABLE IF NOT EXISTS orders (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    order_no TEXT NOT NULL UNIQUE,
    student_id BIGINT NOT NULL REFERENCES student(id),
    course_id BIGINT NOT NULL REFERENCES course(id),
    status TEXT NOT NULL DEFAULT 'pending',
    price_snapshot_cents BIGINT NOT NULL CHECK (price_snapshot_cents >= 0),
    amount_cents BIGINT NOT NULL CHECK (amount_cents >= 0),
    discount_cents BIGINT NOT NULL DEFAULT 0 CHECK (discount_cents >= 0),
    request_id TEXT NOT NULL UNIQUE,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (amount_cents = price_snapshot_cents - discount_cents)
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_pending_order ON orders(student_id, course_id) WHERE status = 'pending';
CREATE TABLE IF NOT EXISTS payment_txn (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    order_id BIGINT NOT NULL REFERENCES orders(id),
    channel TEXT NOT NULL,
    channel_txn_no TEXT NOT NULL,
    amount_cents BIGINT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(channel, channel_txn_no)
);
CREATE TABLE IF NOT EXISTS refund (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    order_id BIGINT NOT NULL REFERENCES orders(id),
    amount_cents BIGINT NOT NULL,
    reason TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS reconcile_record (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    reconcile_date TEXT NOT NULL,
    channel TEXT NOT NULL,
    difference_cents BIGINT NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(reconcile_date, channel)
);
