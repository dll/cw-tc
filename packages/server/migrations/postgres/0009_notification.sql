-- 通知与广播（PRD §7.10 M-2 · 数据字典 §7.1）
CREATE TABLE IF NOT EXISTS broadcast (
    id BIGSERIAL PRIMARY KEY,
    operator_id BIGINT NOT NULL REFERENCES staff(id),
    target TEXT NOT NULL,
    target_value TEXT,
    title TEXT NOT NULL,
    body TEXT NOT NULL,
    link TEXT,
    idempotency_key TEXT NOT NULL,
    recipients_count INTEGER NOT NULL DEFAULT 0,
    sent_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (target IN ('all','role','course','major'))
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_broadcast_idem ON broadcast(idempotency_key);

CREATE TABLE IF NOT EXISTS notification (
    id BIGSERIAL PRIMARY KEY,
    student_id BIGINT NOT NULL REFERENCES student(id),
    type TEXT NOT NULL,
    title TEXT NOT NULL,
    body TEXT NOT NULL,
    link TEXT,
    read_at TIMESTAMPTZ,
    broadcast_id BIGINT REFERENCES broadcast(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (type IN ('system','course','order','exam','cert','broadcast'))
);
CREATE INDEX IF NOT EXISTS idx_notification_student ON notification(student_id, read_at, created_at DESC);
