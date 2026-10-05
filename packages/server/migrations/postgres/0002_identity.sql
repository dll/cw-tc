-- 0002_identity.sql
-- 第三课堂 · 身份与会话迁移（数据字典 §2）
-- 说明：批次 0 先建立可执行骨架；字段以数据字典为唯一来源。
CREATE TABLE IF NOT EXISTS student (
    id BIGSERIAL PRIMARY KEY,
    student_no TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    phone TEXT UNIQUE,
    phone_verified_at TIMESTAMPTZ,
    id_card_hash BYTEA UNIQUE NOT NULL,
    id_card_cipher BYTEA NOT NULL,
    id_card_enc_key_id SMALLINT NOT NULL DEFAULT 1,
    password_hash TEXT,
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS staff (
    id BIGSERIAL PRIMARY KEY,
    account TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('teacher','admin_ops','admin_super')),
    password_hash TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS student_session (
    id BIGSERIAL PRIMARY KEY,
    student_id BIGINT NOT NULL REFERENCES student(id),
    token_hash BYTEA NOT NULL UNIQUE,
    issued_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ
);
CREATE TABLE IF NOT EXISTS sms_code (
    id BIGSERIAL PRIMARY KEY,
    phone TEXT NOT NULL,
    scene TEXT NOT NULL,
    code_hash BYTEA NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
