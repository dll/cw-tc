-- 0004_commerce（SQLite · 仅供开发与测试 方言）
-- 第三课堂 · 权益、订单与资金（数据字典 §4）
-- ⚠️ 本文件由数据字典重建（v2.25）：表与列**逐字段对齐字典 §2~§5**。
-- ⚠️ 字典是**终态**；**本批次只建基列**，后续迁移新增的列见每个表下的 `@deferred` 行
--    （`tools/audit_prd_closure.py` 会校验：**基列 ∪ deferred == 字典列**，不多不少）。
-- ⚠️ 可直接重建的理由：**这些迁移尚未在任何持久环境执行过**（生产未部署；dev/test 每次从空库重建）。

CREATE TABLE IF NOT EXISTS orders (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    order_no TEXT NOT NULL,
    student_id INTEGER NOT NULL REFERENCES student(id),
    course_id INTEGER NOT NULL REFERENCES course(id),
    course_name_snapshot TEXT NOT NULL,
    price_snapshot_cents INTEGER NOT NULL,
    amount_cents INTEGER NOT NULL,
    status TEXT NOT NULL,
    channel TEXT,
    request_id TEXT,
    expired_at TEXT NOT NULL,
    paid_at TEXT,
    closed_at TEXT,
    refunded_at TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (price_snapshot_cents >= 0),
    CHECK (amount_cents >= 0),
    CHECK (channel IS NULL OR channel IN ('wechat','alipay'))
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_orders_order_no ON orders(order_no);
CREATE UNIQUE INDEX IF NOT EXISTS uq_pending_order ON orders(student_id, course_id) WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_orders_student ON orders(student_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_orders_status ON orders(status, created_at);
CREATE INDEX IF NOT EXISTS idx_orders_channel_paid ON orders(channel, paid_at);
-- @deferred orders: discount_cents@0019 list_amount_cents@0019 promo_code@0019
-- @note orders: 0019：补 `CHECK (amount_cents = price_snapshot_cents - discount_cents)`（依赖 discount_cents，必须同批加）
-- @note orders: ⚠️ 字典里表名写作 `order`（**PG 保留字**，直接建表会失败）→ 本文件用 `orders`，字典已同步改名

CREATE TABLE IF NOT EXISTS entitlement (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    student_id INTEGER NOT NULL REFERENCES student(id),
    course_id INTEGER NOT NULL REFERENCES course(id),
    source TEXT NOT NULL,
    order_id INTEGER REFERENCES orders(id),
    granted_by INTEGER REFERENCES staff(id),
    grant_reason TEXT,
    granted_at TEXT NOT NULL,
    revoked_at TEXT,
    revoke_reason TEXT,
    CHECK (source IN ('order','free','grant')),
    CHECK (revoke_reason IS NULL OR revoke_reason IN ('order_refunded','admin')),
    CHECK (source <> 'order' OR order_id IS NOT NULL),
    CHECK (source <> 'grant' OR (granted_by IS NOT NULL AND grant_reason IS NOT NULL))
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_entitlement_active ON entitlement(student_id, course_id) WHERE revoked_at IS NULL;

CREATE TABLE IF NOT EXISTS payment_txn (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    order_id INTEGER NOT NULL REFERENCES orders(id),
    channel TEXT NOT NULL,
    channel_txn_id TEXT NOT NULL,
    amount_cents INTEGER NOT NULL,
    raw_status TEXT,
    idempotency_key TEXT NOT NULL,
    callback_payload TEXT NOT NULL,
    verified_at TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_payment_txn_channel ON payment_txn(channel, channel_txn_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_payment_txn_idem ON payment_txn(idempotency_key);

CREATE TABLE IF NOT EXISTS refund (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    refund_no TEXT NOT NULL,
    order_id INTEGER NOT NULL REFERENCES orders(id),
    kind TEXT NOT NULL,
    basis TEXT,
    basis_ref TEXT,
    amount_cents INTEGER NOT NULL,
    reason TEXT NOT NULL,
    initiator TEXT NOT NULL,
    operator_id INTEGER REFERENCES staff(id),
    channel_refund_id TEXT,
    status TEXT NOT NULL DEFAULT 'processing',
    fail_reason TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    finished_at TEXT,
    CHECK (kind IN ('normal','reward')),
    CHECK (basis IS NULL OR basis IN ('score','campaign','other')),
    CHECK (amount_cents > 0),
    CHECK (initiator IN ('student','admin')),
    CHECK (status IN ('processing','success','failed')),
    CHECK (kind <> 'reward' OR (basis IS NOT NULL AND basis_ref IS NOT NULL)),
    CHECK (initiator <> 'admin' OR operator_id IS NOT NULL)
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_refund_no ON refund(refund_no);

CREATE TABLE IF NOT EXISTS reconcile_record (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    biz_date DATE NOT NULL,
    channel TEXT NOT NULL,
    channel_count INT NOT NULL,
    channel_amount_cents INTEGER NOT NULL,
    local_count INT NOT NULL,
    local_amount_cents INTEGER NOT NULL,
    diff_count INT NOT NULL,
    diff_amount_cents INTEGER NOT NULL,
    detail TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_reconcile_biz_date ON reconcile_record(biz_date, channel);
