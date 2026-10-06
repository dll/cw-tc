-- 0005_exam_cert（PostgreSQL 方言）
-- 第三课堂 · 测评与证书（数据字典 §5）
-- ⚠️ 本文件由数据字典重建（v2.25）：表与列**逐字段对齐字典 §2~§5**。
-- ⚠️ 字典是**终态**；**本批次只建基列**，后续迁移新增的列见每个表下的 `@deferred` 行
--    （`tools/audit_prd_closure.py` 会校验：**基列 ∪ deferred == 字典列**，不多不少）。
-- ⚠️ 可直接重建的理由：**这些迁移尚未在任何持久环境执行过**（生产未部署；dev/test 每次从空库重建）。

CREATE TABLE IF NOT EXISTS exam_session (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    student_id BIGINT NOT NULL REFERENCES student(id),
    course_id BIGINT NOT NULL REFERENCES course(id),
    level TEXT NOT NULL,
    paper JSONB NOT NULL,
    answers JSONB,
    status TEXT NOT NULL,
    started_at TIMESTAMPTZ,
    submitted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (level IN ('L1','L2','L3')),
    CHECK (status IN ('ongoing','submitted','graded'))
);
CREATE INDEX IF NOT EXISTS idx_exam_session_student ON exam_session(student_id, created_at DESC);
-- @deferred exam_session: active_device_key@0019 anomaly_count@0019 last_heartbeat_at@0019 retake_seq@0019 server_deadline_at@0019
-- @note exam_session: 0019：补**部分唯一索引** `UNIQUE(student_id, course_id, level) WHERE status='ongoing'`（同档单会话）

CREATE TABLE IF NOT EXISTS exam_result (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    exam_session_id BIGINT NOT NULL REFERENCES exam_session(id),
    student_id BIGINT REFERENCES student(id),
    course_id BIGINT REFERENCES course(id),
    level TEXT NOT NULL,
    auto_score NUMERIC(6,1) NOT NULL DEFAULT 0,
    manual_score NUMERIC(6,1) NOT NULL DEFAULT 0,
    total_score NUMERIC(6,1) NOT NULL DEFAULT 0,
    band TEXT,
    is_pass BOOLEAN NOT NULL DEFAULT false,
    graded_at TIMESTAMPTZ,
    detail JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (band IS NULL OR band IN ('pass','good','excellent','fail'))
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_exam_result_session ON exam_result(exam_session_id);
CREATE INDEX IF NOT EXISTS idx_exam_result_rank ON exam_result(student_id, course_id, level, total_score DESC);

CREATE TABLE IF NOT EXISTS manual_review (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    exam_result_id BIGINT NOT NULL REFERENCES exam_result(id),
    question_id BIGINT NOT NULL REFERENCES question(id),
    reviewer_id BIGINT REFERENCES staff(id),
    points JSONB NOT NULL,
    score NUMERIC(6,1) NOT NULL,
    comment TEXT,
    is_second_review BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_manual_review ON manual_review(exam_result_id, question_id, reviewer_id);

CREATE TABLE IF NOT EXISTS certificate (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    cert_no TEXT NOT NULL,
    student_id BIGINT NOT NULL REFERENCES student(id),
    course_id BIGINT NOT NULL REFERENCES course(id),
    level TEXT NOT NULL,
    score NUMERIC(6,1) NOT NULL,
    band TEXT NOT NULL,
    issuer TEXT NOT NULL DEFAULT '滁州畅网智能科技有限公司',
    issued_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    pdf_storage_key TEXT,
    pdf_sha256 TEXT,
    status TEXT NOT NULL DEFAULT 'valid',
    revoked_reason TEXT,
    revoked_at TIMESTAMPTZ,
    revoked_by BIGINT REFERENCES staff(id),
    issued_by BIGINT REFERENCES staff(id),
    id_card_mode TEXT NOT NULL DEFAULT 'mask',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (level IN ('L1','L2','L3')),
    CHECK (status IN ('valid','revoked')),
    CHECK (revoked_reason IS NULL OR revoked_reason IN ('admin','order_refunded')),
    CHECK (id_card_mode IN ('mask','full'))
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_certificate_no ON certificate(cert_no);
CREATE UNIQUE INDEX IF NOT EXISTS uq_certificate_active ON certificate(student_id, course_id, level) WHERE revoked_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_certificate_expires ON certificate(expires_at) WHERE status = 'valid';

CREATE TABLE IF NOT EXISTS cert_no_sequence (
    course_code TEXT NOT NULL,
    level_digit TEXT NOT NULL,
    year SMALLINT NOT NULL,
    last_seq INT NOT NULL DEFAULT 0,
    PRIMARY KEY (course_code, level_digit, year),
    CHECK (level_digit IN ('A','M','H'))
);
