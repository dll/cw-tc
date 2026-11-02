-- 批量操作结果（PRD §7.10.8）：先预览，执行后逐条留痕
CREATE TABLE IF NOT EXISTS batch_job (
    id TEXT PRIMARY KEY,
    action TEXT NOT NULL,
    operator_id INTEGER NOT NULL REFERENCES staff(id),
    preview INTEGER NOT NULL DEFAULT false,
    results TEXT NOT NULL DEFAULT '[]',
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);