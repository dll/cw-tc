-- 0002_identity.sql（**SQLite 方言 · 仅供开发与测试**）
-- 与 `migrations/postgres/0002_identity.sql` 的表与列**一一对应**；差异仅类型与自增写法。
-- ⚠️ 生产不使用本文件；上线走 postgres/ 那一套（见 migrations/README.md）。
-- 本文件由 `migrations/postgres/0002_identity.sql` 机械翻译：BIGSERIAL PRIMARY KEY×4 · TIMESTAMPTZ×10 · BYTEA×4 · DEFAULT now()×4
CREATE TABLE IF NOT EXISTS student (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    student_no TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    phone TEXT UNIQUE,
    phone_verified_at TEXT,
    id_card_hash BLOB UNIQUE NOT NULL,
    id_card_cipher BLOB NOT NULL,
    id_card_enc_key_id SMALLINT NOT NULL DEFAULT 1,
    password_hash TEXT,
    status TEXT NOT NULL DEFAULT 'active',
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS staff (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    account TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('teacher','admin_ops','admin_super')),
    password_hash TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active',
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS student_session (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    student_id BIGINT NOT NULL REFERENCES student(id),
    token_hash BLOB NOT NULL UNIQUE,
    issued_at TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    revoked_at TEXT
);
CREATE TABLE IF NOT EXISTS sms_code (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    phone TEXT NOT NULL,
    scene TEXT NOT NULL,
    code_hash BLOB NOT NULL,
    expires_at TEXT NOT NULL,
    consumed_at TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
