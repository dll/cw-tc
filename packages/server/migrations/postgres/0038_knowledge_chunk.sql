-- 课程知识库切片（PRD §7.5 H-2）
-- 开发/测试用 SQLite 文本检索兜底；生产 PostgreSQL 可后续加 pgvector 列，本表仍保存原文与出处。
CREATE TABLE IF NOT EXISTS knowledge_chunk (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    course_code TEXT NOT NULL REFERENCES course(course_code),
    lecture_no INTEGER NOT NULL,
    lecture_title TEXT NOT NULL,
    asset_id BIGINT NOT NULL REFERENCES asset(id) ON DELETE CASCADE,
    asset_title TEXT NOT NULL,
    storage_key TEXT NOT NULL,
    chunk_index INTEGER NOT NULL,
    content TEXT NOT NULL,
    char_count INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(asset_id, chunk_index)
);
CREATE INDEX IF NOT EXISTS idx_knowledge_chunk_course ON knowledge_chunk(course_code, lecture_no);