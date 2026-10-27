CREATE TABLE IF NOT EXISTS student_course_discount (
    id BIGSERIAL PRIMARY KEY,
    student_id BIGINT NOT NULL REFERENCES student(id),
    course_id BIGINT NOT NULL REFERENCES course(id),
    discount_cents BIGINT NOT NULL,
    reason TEXT NOT NULL,
    granted_by BIGINT NOT NULL REFERENCES staff(id),
    active INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (discount_cents >= 0)
);
ALTER TABLE orders ADD COLUMN IF NOT EXISTS discount_cents BIGINT NOT NULL DEFAULT 0;
CREATE UNIQUE INDEX IF NOT EXISTS uq_student_course_discount_active ON student_course_discount(student_id, course_id) WHERE active = 1;
