-- 对外展示（PRD §7.17 / 数据字典 §6.10）
CREATE TABLE IF NOT EXISTS web_content (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    block TEXT NOT NULL,
    slot INTEGER NOT NULL DEFAULT 0,
    title TEXT,
    body TEXT NOT NULL,
    evidence_ref TEXT,
    version INTEGER NOT NULL DEFAULT 1,
    status TEXT NOT NULL DEFAULT 'draft',
    published_by BIGINT REFERENCES staff(id),
    published_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (block IN ('intro_short','intro_para','intro_long','highlight','ai_tech','major_note','claim_note','filing_note')),
    CHECK (status IN ('draft','published','retired')),
    CHECK (status <> 'published' OR (evidence_ref IS NOT NULL AND evidence_ref <> '')),
    CHECK (block <> 'filing_note' OR status <> 'published' OR (body IS NOT NULL AND body <> ''))
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_web_content_published ON web_content(block, slot) WHERE status='published';
CREATE INDEX IF NOT EXISTS idx_web_content_block ON web_content(block, slot, version DESC);

CREATE TABLE IF NOT EXISTS job_map (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    course_code TEXT NOT NULL REFERENCES course(course_code),
    cert_level TEXT NOT NULL,
    job_direction TEXT NOT NULL,
    skill_items JSONB NOT NULL DEFAULT '[]'::jsonb,
    lecture_refs JSONB NOT NULL DEFAULT '[]'::jsonb,
    evidence_ref TEXT,
    status TEXT NOT NULL DEFAULT 'draft',
    reviewed_by BIGINT REFERENCES staff(id),
    reviewed_at TIMESTAMPTZ,
    major_id BIGINT REFERENCES major(id),
    CHECK (cert_level IN ('L1','L2','L3')),
    CHECK (status IN ('draft','published','retired')),
    CHECK (status <> 'published' OR (evidence_ref IS NOT NULL AND evidence_ref <> '' AND jsonb_array_length(lecture_refs) > 0))
);
CREATE INDEX IF NOT EXISTS idx_job_map_course ON job_map(course_code, cert_level);
CREATE INDEX IF NOT EXISTS idx_job_map_status ON job_map(status);

CREATE TABLE IF NOT EXISTS showcase_item (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    student_id BIGINT NOT NULL REFERENCES student(id),
    kind TEXT NOT NULL,
    nickname TEXT NOT NULL,
    major_id BIGINT REFERENCES major(id),
    cert_no TEXT,
    cert_level TEXT,
    summary TEXT NOT NULL,
    job_direction TEXT,
    company_name TEXT,
    company_consent BOOLEAN NOT NULL DEFAULT FALSE,
    contest_name TEXT,
    contest_year INTEGER,
    award_level TEXT,
    proof_no TEXT,
    team_role TEXT,
    evidence_kind TEXT,
    verified_by BIGINT REFERENCES staff(id),
    verified_at TIMESTAMPTZ,
    display_consent_at TIMESTAMPTZ,
    consent_revoked_at TIMESTAMPTZ,
    status TEXT NOT NULL DEFAULT 'self_report',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (kind IN ('job_landed','contest_award','cert_earned','review_quote')),
    CHECK (status IN ('self_report','verified','withdrawn')),
    CHECK (kind <> 'contest_award' OR (contest_name IS NOT NULL AND proof_no IS NOT NULL)),
    CHECK (company_name IS NULL OR company_consent = TRUE)
);
CREATE INDEX IF NOT EXISTS idx_showcase_public ON showcase_item(status, display_consent_at, consent_revoked_at);
CREATE INDEX IF NOT EXISTS idx_showcase_student ON showcase_item(student_id, created_at DESC);

INSERT INTO system_config(key, value, value_type, min_value, max_value, description, updated_at)
VALUES ('site.showcase_revoke_hours', '24', 'int', 24, NULL, '学员撤回成果授权后全站下架时限（小时）', now())
ON CONFLICT (key) DO NOTHING;