CREATE TABLE IF NOT EXISTS student_course_discount (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    student_id INTEGER NOT NULL REFERENCES student(id),
    course_id INTEGER NOT NULL REFERENCES course(id),
    discount_cents INTEGER NOT NULL,
    reason TEXT NOT NULL,
    granted_by INTEGER NOT NULL REFERENCES staff(id),
    active INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (discount_cents >= 0)
);
ALTER TABLE orders ADD COLUMN discount_cents INTEGER NOT NULL DEFAULT 0;
CREATE UNIQUE INDEX IF NOT EXISTS uq_student_course_discount_active ON student_course_discount(student_id, course_id) WHERE active = 1;
