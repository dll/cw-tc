ALTER TABLE student ADD COLUMN registration_reviewed_at TIMESTAMPTZ;
ALTER TABLE student ADD COLUMN registration_reviewed_by BIGINT REFERENCES staff(id);
ALTER TABLE student ADD COLUMN registration_review_status TEXT NOT NULL DEFAULT 'approved';
CREATE INDEX IF NOT EXISTS idx_student_registration_review ON student(registration_reviewed_at, created_at DESC);
