-- 数据导出留痕（PRD §7.10.9 / 数据字典 §7.4）
-- 任何导出都必须有记录；代导出仅 super，内容一律脱敏。
CREATE TABLE IF NOT EXISTS data_export (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    subject_type TEXT NOT NULL,
    subject_id BIGINT NOT NULL,
    initiator TEXT NOT NULL,
    operator_id BIGINT REFERENCES staff(id),
    reason TEXT,
    scope TEXT NOT NULL,
    format TEXT NOT NULL,
    file_ref TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (subject_type = 'student'),
    CHECK (initiator IN ('self','admin')),
    CHECK (scope IN ('account','learning','scores','certs','notes','all')),
    CHECK (format IN ('json','md','zip')),
    CHECK (initiator <> 'admin' OR (operator_id IS NOT NULL AND reason IS NOT NULL AND reason <> ''))
);
CREATE INDEX IF NOT EXISTS idx_data_export_subject ON data_export(subject_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_data_export_operator ON data_export(operator_id, created_at DESC);
