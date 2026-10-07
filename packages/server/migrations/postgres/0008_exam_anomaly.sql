-- 0008_exam_anomaly（PostgreSQL 方言）
-- 第三课堂 · 测评异常记录（数据字典 §5.1.1 · PRD §7.7 C-7 · 铁律 15）
-- 只记信号，不带判定：verdict 默认 pending，不影响成绩；判定必须由人做（不得 AI 判定作弊）。
CREATE TABLE IF NOT EXISTS exam_anomaly (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    exam_id BIGINT NOT NULL REFERENCES exam_session(id),
    kind TEXT NOT NULL,
    detail JSONB,
    at TIMESTAMPTZ NOT NULL DEFAULT now(),
    verdict TEXT NOT NULL DEFAULT 'pending',
    verdict_by BIGINT REFERENCES staff(id),
    verdict_at TIMESTAMPTZ,
    verdict_reason TEXT,
    CHECK (kind IN ('blur', 'too_fast', 'same_option_streak', 'device_change', 'ip_change', 'multi_session', 'no_heartbeat')),
    CHECK (verdict IN ('pending', 'kept', 'invalid', 'manual_check'))
);
CREATE INDEX IF NOT EXISTS idx_exam_anomaly_exam ON exam_anomaly(exam_id, kind);
