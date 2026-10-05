-- 0005_exam_cert.sql
-- 第三课堂 · 测评与证书迁移（数据字典 §5）
CREATE TABLE IF NOT EXISTS exam_session (
    id BIGSERIAL PRIMARY KEY,
    student_id BIGINT NOT NULL REFERENCES student(id),
    course_id BIGINT NOT NULL REFERENCES course(id),
    level TEXT NOT NULL CHECK (level IN ('L1','L2','L3')),
    answers JSONB NOT NULL DEFAULT '{}'::jsonb,
    status TEXT NOT NULL DEFAULT 'ongoing' CHECK (status IN ('ongoing','submitted','graded')),
    started_at TIMESTAMPTZ NOT NULL,
    submitted_at TIMESTAMPTZ,
    server_deadline_at TIMESTAMPTZ NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_exam_ongoing ON exam_session(student_id, course_id, level) WHERE status = 'ongoing';
CREATE TABLE IF NOT EXISTS exam_result (
    id BIGSERIAL PRIMARY KEY,
    exam_session_id BIGINT NOT NULL REFERENCES exam_session(id),
    score NUMERIC(6,1) NOT NULL,
    passed BOOLEAN NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS manual_review (
    id BIGSERIAL PRIMARY KEY,
    exam_session_id BIGINT NOT NULL REFERENCES exam_session(id),
    reviewer_id BIGINT REFERENCES staff(id),
    verdict TEXT NOT NULL,
    reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS cert_no_sequence (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    next_value BIGINT NOT NULL DEFAULT 1
);
CREATE TABLE IF NOT EXISTS certificate (
    id BIGSERIAL PRIMARY KEY,
    certificate_no TEXT NOT NULL UNIQUE,
    student_id BIGINT NOT NULL REFERENCES student(id),
    course_id BIGINT NOT NULL REFERENCES course(id),
    level TEXT NOT NULL CHECK (level IN ('L1','L2','L3')),
    issued_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_certificate_active ON certificate(student_id, course_id, level) WHERE revoked_at IS NULL;
