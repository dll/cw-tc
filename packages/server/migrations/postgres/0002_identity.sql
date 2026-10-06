-- 0002_identity（PostgreSQL 方言）
-- 第三课堂 · 账号与实名（数据字典 §2）
-- ⚠️ 本文件由数据字典重建（v2.25）：表与列**逐字段对齐字典 §2~§5**。
-- ⚠️ 字典是**终态**；**本批次只建基列**，后续迁移新增的列见每个表下的 `@deferred` 行
--    （`tools/audit_prd_closure.py` 会校验：**基列 ∪ deferred == 字典列**，不多不少）。
-- ⚠️ 可直接重建的理由：**这些迁移尚未在任何持久环境执行过**（生产未部署；dev/test 每次从空库重建）。

CREATE TABLE IF NOT EXISTS student (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    student_no TEXT NOT NULL,
    name TEXT NOT NULL,
    phone TEXT NOT NULL,
    phone_verified_at TIMESTAMPTZ,
    id_card_hash BYTEA NOT NULL,
    id_card_cipher BYTEA NOT NULL,
    id_card_enc_key_id SMALLINT NOT NULL DEFAULT 1,
    password_hash TEXT,
    password_updated_at TIMESTAMPTZ,
    status TEXT NOT NULL DEFAULT 'active',
    failed_login_count INT NOT NULL DEFAULT 0,
    locked_until TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_login_at TIMESTAMPTZ,
    agreement_versions JSONB NOT NULL,
    CHECK (status IN ('active','limited','suspended','banned','deleting','anonymized'))
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_student_phone ON student(phone);
CREATE UNIQUE INDEX IF NOT EXISTS uq_student_id_card_hash ON student(id_card_hash);
CREATE UNIQUE INDEX IF NOT EXISTS uq_student_no ON student(student_no);
CREATE INDEX IF NOT EXISTS idx_student_created ON student(created_at);
-- @deferred student: status_by@0010 status_note@0010 status_reason_kind@0010 status_until@0010
-- @note student: 0010：`student.phone` 的普通唯一索引改为**部分唯一**（`WHERE phone IS NOT NULL AND phone <> ''`，注销后置空）

CREATE TABLE IF NOT EXISTS student_session (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    student_id BIGINT NOT NULL REFERENCES student(id),
    token_hash BYTEA NOT NULL,
    issued_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    user_agent TEXT,
    ip INET,
    revoked_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_student_session_token ON student_session(token_hash);
-- @deferred student_session: device_name@0009 device_type@0009 last_active_at@0009

CREATE TABLE IF NOT EXISTS staff (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    account TEXT NOT NULL,
    name TEXT NOT NULL,
    role TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    password_changed_at TIMESTAMPTZ,
    created_source TEXT NOT NULL,
    created_by BIGINT REFERENCES staff(id),
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (role IN ('teacher','admin_ops','admin_super')),
    CHECK (created_source IN ('seed','admin')),
    CHECK (status IN ('active','disabled'))
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_staff_account ON staff(account);
-- @deferred staff: phone@0009

CREATE TABLE IF NOT EXISTS sms_code (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    phone TEXT NOT NULL,
    scene TEXT NOT NULL,
    code_hash BYTEA NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    ip INET,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (scene IN ('register','login','reset','rebind','sensitive'))
);
CREATE INDEX IF NOT EXISTS idx_sms_code_phone ON sms_code(phone, scene, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_sms_code_ip ON sms_code(ip, created_at DESC);
