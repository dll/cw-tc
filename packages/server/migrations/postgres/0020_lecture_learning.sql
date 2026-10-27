-- 每讲练习与讨论，独立于课程总测评及证书。
CREATE TABLE IF NOT EXISTS lecture_question (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    lecture_id BIGINT NOT NULL REFERENCES lecture(id) ON DELETE CASCADE,
    source_key TEXT NOT NULL,
    prompt TEXT NOT NULL,
    reference_answer TEXT,
    evidence_ref TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(lecture_id, source_key)
);
CREATE TABLE IF NOT EXISTS lecture_answer (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    question_id BIGINT NOT NULL REFERENCES lecture_question(id) ON DELETE CASCADE,
    student_id BIGINT NOT NULL REFERENCES student(id),
    answer TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_lecture_answer_student ON lecture_answer(student_id, question_id, created_at);

CREATE TABLE IF NOT EXISTS lecture_topic (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    lecture_id BIGINT NOT NULL REFERENCES lecture(id) ON DELETE CASCADE,
    student_id BIGINT NOT NULL REFERENCES student(id),
    body TEXT NOT NULL,
    like_count INT NOT NULL DEFAULT 0 CHECK(like_count >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_lecture_topic_rank ON lecture_topic(lecture_id, like_count DESC, created_at DESC, id DESC);
CREATE TABLE IF NOT EXISTS lecture_reply (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    topic_id BIGINT NOT NULL REFERENCES lecture_topic(id) ON DELETE CASCADE,
    student_id BIGINT NOT NULL REFERENCES student(id),
    body TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_lecture_reply_topic ON lecture_reply(topic_id, created_at, id);
CREATE TABLE IF NOT EXISTS lecture_topic_like (
    topic_id BIGINT NOT NULL REFERENCES lecture_topic(id) ON DELETE CASCADE,
    student_id BIGINT NOT NULL REFERENCES student(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY(topic_id, student_id)
);
