-- 0004_commerce.sql
-- 第三课堂 · 交易与权益迁移（数据字典 §4）
--
-- ⚠️ v2.23 修订：本文件此前与数据字典 §4.1 不一致（`entitlement` 少了
--    `source` / `order_id` / `granted_at` / `revoke_reason`，还多了字典里没有的 `status`）。
--    数据字典是**建表唯一依据**，故此处以字典为准补齐，并新增 G-7 需要的
--    `granted_by` / `grant_reason`。
--    可直接修订的理由：**本迁移尚未在任何持久环境执行过**（生产未部署；dev/test 每次从空库重建），
--    不存在"改了不会重跑、环境之间产生看不见差异"的风险。**若已执行过，则必须新增编号而非修改**。
--
-- 顺序说明：`entitlement` 引用 `orders`，故 `orders` 必须**先建**（PostgreSQL 要求 FK 目标表已存在）。

CREATE TABLE IF NOT EXISTS orders (
    id BIGSERIAL PRIMARY KEY,
    order_no TEXT NOT NULL UNIQUE,
    student_id BIGINT NOT NULL REFERENCES student(id),
    course_id BIGINT NOT NULL REFERENCES course(id),
    status TEXT NOT NULL DEFAULT 'pending',
    price_snapshot_cents BIGINT NOT NULL CHECK (price_snapshot_cents >= 0),
    amount_cents BIGINT NOT NULL CHECK (amount_cents >= 0),
    discount_cents BIGINT NOT NULL DEFAULT 0 CHECK (discount_cents >= 0),
    request_id TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (amount_cents = price_snapshot_cents - discount_cents)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_pending_order ON orders(student_id, course_id) WHERE status = 'pending';

-- 学习权益：三种来源 —— 付费（order）/ 免费课（free）/ 管理员授予（grant，PRD §7.3 G-7）
CREATE TABLE IF NOT EXISTS entitlement (
    id BIGSERIAL PRIMARY KEY,
    student_id BIGINT NOT NULL REFERENCES student(id),
    course_id BIGINT NOT NULL REFERENCES course(id),
    source TEXT NOT NULL CHECK (source IN ('order','free','grant')),
    order_id BIGINT REFERENCES orders(id),
    granted_by BIGINT REFERENCES staff(id),
    grant_reason TEXT,
    granted_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    revoke_reason TEXT CHECK (revoke_reason IS NULL OR revoke_reason IN ('order_refunded','admin')),
    -- 付费权益必须挂订单；管理员授予必须留"谁授的 + 为什么"（不得留空，否则等同无痕）
    CHECK (source <> 'order' OR order_id IS NOT NULL),
    CHECK (source <> 'grant' OR (granted_by IS NOT NULL AND grant_reason IS NOT NULL))
);

-- "当前有效权益唯一"：一门课同一时刻只能有一条未撤销的权益（先授权后付费也必须复用同一条）
CREATE UNIQUE INDEX IF NOT EXISTS uq_entitlement_active ON entitlement(student_id, course_id) WHERE revoked_at IS NULL;

CREATE TABLE IF NOT EXISTS payment_txn (
    id BIGSERIAL PRIMARY KEY,
    order_id BIGINT NOT NULL REFERENCES orders(id),
    channel TEXT NOT NULL,
    channel_txn_no TEXT NOT NULL,
    amount_cents BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(channel, channel_txn_no)
);

CREATE TABLE IF NOT EXISTS refund (
    id BIGSERIAL PRIMARY KEY,
    order_id BIGINT NOT NULL REFERENCES orders(id),
    amount_cents BIGINT NOT NULL,
    reason TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS reconcile_record (
    id BIGSERIAL PRIMARY KEY,
    reconcile_date DATE NOT NULL,
    channel TEXT NOT NULL,
    difference_cents BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(reconcile_date, channel)
);
