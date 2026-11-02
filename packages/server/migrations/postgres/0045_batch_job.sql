-- 批量操作结果（PRD §7.10.8）：先预览，执行后逐条留痕
CREATE TABLE IF NOT EXISTS batch_job (
    id TEXT PRIMARY KEY,
    action TEXT NOT NULL,
    operator_id BIGINT NOT NULL REFERENCES staff(id),
    preview BOOLEAN NOT NULL DEFAULT FALSE,
    results JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);