-- 学习笔记 / 建议 / 评价举报（PRD §7.6 I、§7.13 L-8/L-9、§7.14 M-3）
CREATE TABLE IF NOT EXISTS note (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    student_id BIGINT NOT NULL REFERENCES student(id),
    course_id BIGINT NOT NULL REFERENCES course(id),
    lecture_no INTEGER,
    title TEXT NOT NULL,
    content_md TEXT NOT NULL,
    source TEXT NOT NULL DEFAULT 'manual',
    source_ref JSONB,
    tags TEXT[],
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_note_student ON note(student_id, course_id, updated_at DESC);

CREATE TABLE IF NOT EXISTS suggestion (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    student_id BIGINT NOT NULL REFERENCES student(id),
    course_code TEXT REFERENCES course(course_code),
    body TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','accepted','rejected','planned')),
    reply TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_suggestion_student ON suggestion(student_id, created_at DESC);

CREATE TABLE IF NOT EXISTS comment_report (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    comment_id BIGINT NOT NULL REFERENCES course_comment(id),
    reporter_id BIGINT NOT NULL REFERENCES student(id),
    reason_kind TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','handled')),
    handled_by BIGINT REFERENCES staff(id),
    handled_at TIMESTAMPTZ,
    result_note TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(comment_id, reporter_id)
);
