-- 学员增长与助学（PRD §7.14 M-2 / §7.14 M-3 / §7.5 H-1）
CREATE TABLE IF NOT EXISTS favorite (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    student_id INTEGER NOT NULL REFERENCES student(id),
    course_code TEXT NOT NULL REFERENCES course(course_code),
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(student_id, course_code)
);
CREATE INDEX IF NOT EXISTS idx_favorite_student ON favorite(student_id, created_at DESC);

CREATE TABLE IF NOT EXISTS course_comment (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    course_code TEXT NOT NULL REFERENCES course(course_code),
    student_id INTEGER NOT NULL REFERENCES student(id),
    rating INTEGER NOT NULL CHECK (rating BETWEEN 1 AND 5),
    rating_content INTEGER CHECK (rating_content BETWEEN 1 AND 5),
    rating_level INTEGER CHECK (rating_level BETWEEN 1 AND 5),
    rating_clarity INTEGER CHECK (rating_clarity BETWEEN 1 AND 5),
    lecture_no INTEGER,
    body TEXT,
    completed_lectures INTEGER NOT NULL DEFAULT 0,
    total_lectures INTEGER NOT NULL DEFAULT 0,
    nickname_shown TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','visible','removed')),
    remove_reason_kind TEXT,
    edited_at TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(student_id, course_code),
    CHECK (body IS NULL OR length(body) BETWEEN 20 AND 500),
    CHECK (status <> 'removed' OR remove_reason_kind IS NOT NULL)
);
CREATE INDEX IF NOT EXISTS idx_course_comment_course ON course_comment(course_code, status, created_at DESC);

CREATE TABLE IF NOT EXISTS ai_daily_quota (
    student_id INTEGER NOT NULL REFERENCES student(id),
    course_id INTEGER NOT NULL REFERENCES course(id),
    stat_date TEXT NOT NULL,
    used INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (student_id, course_id, stat_date)
);
