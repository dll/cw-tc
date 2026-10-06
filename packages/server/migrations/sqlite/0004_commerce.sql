-- 0004_commerce.sql（**SQLite 方言 · 仅供开发与测试**）
-- 与 `migrations/postgres/0004_commerce.sql` 的表与列**一一对应**；差异仅类型与自增写法。
-- ⚠️ 生产不使用本文件；上线走 postgres/ 那一套（见 migrations/README.md）。
-- 本文件由 `migrations/postgres/0004_commerce.sql` 机械翻译：BIGSERIAL PRIMARY KEY×5 · TIMESTAMPTZ×6 · DEFAULT now()×4 · reconcile_date DATE×1

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

-- 学习权益：三种来源 —— 付费（order）/ 免费课（free）/ 管理员授予（grant，PRD §7.3 G-7）
CREATE TABLE IF NOT EXISTS entitlement (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    student_id BIGINT NOT NULL REFERENCES student(id),
    course_id BIGINT NOT NULL REFERENCES course(id),
    source TEXT NOT NULL CHECK (source IN ('order','free','grant')),
    order_id BIGINT REFERENCES orders(id),
    granted_by BIGINT REFERENCES staff(id),
    grant_reason TEXT,
    granted_at TEXT NOT NULL,
    revoked_at TEXT,
    revoke_reason TEXT CHECK (revoke_reason IS NULL OR revoke_reason IN ('order_refunded','admin')),
    -- 付费权益必须挂订单；管理员授予必须留"谁授的 + 为什么"（不得留空，否则等同无痕）
    CHECK (source <> 'order' OR order_id IS NOT NULL),
    CHECK (source <> 'grant' OR (granted_by IS NOT NULL AND grant_reason IS NOT NULL))
);

-- "当前有效权益唯一"：一门课同一时刻只能有一条未撤销的权益（先授权后付费也必须复用同一条）
CREATE UNIQUE INDEX IF NOT EXISTS uq_entitlement_active ON entitlement(student_id, course_id) WHERE revoked_at IS NULL;

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
