-- 课程统计与业务参数（PRD §7.14 M-0 / §7.9 J-10 · 数据字典 §6.6 / §6.7）
-- 统计口径：按人去重、T+1 跑批、幂等 ON CONFLICT DO UPDATE、人数低于阈值对外隐藏。

-- 访问埋点：仅登录学员记录（匿名无法按人去重）；同一人同课 24h 内只记一次。
CREATE TABLE IF NOT EXISTS course_view (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    course_code TEXT NOT NULL REFERENCES course(course_code),
    student_id INTEGER REFERENCES student(id),
    viewed_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_course_view_course ON course_view(course_code, viewed_at);
CREATE INDEX IF NOT EXISTS idx_course_view_student ON course_view(student_id, course_code, viewed_at);

-- 课程日统计：复合主键 (course_code, stat_date)；同一天重复跑批结果必须一致。
CREATE TABLE IF NOT EXISTS course_stat_daily (
    course_code TEXT NOT NULL REFERENCES course(course_code),
    stat_date TEXT NOT NULL,
    views_uv INTEGER NOT NULL DEFAULT 0,
    favorites INTEGER NOT NULL DEFAULT 0,
    learning INTEGER NOT NULL DEFAULT 0,
    completed INTEGER NOT NULL DEFAULT 0,
    examined INTEGER NOT NULL DEFAULT 0,
    certified INTEGER NOT NULL DEFAULT 0,
    orders INTEGER NOT NULL DEFAULT 0,
    paid INTEGER NOT NULL DEFAULT 0,
    unlock INTEGER NOT NULL DEFAULT 0,
    avg_rating REAL,
    rating_count INTEGER NOT NULL DEFAULT 0,
    computed_at TEXT NOT NULL,
    PRIMARY KEY (course_code, stat_date),
    CHECK (views_uv >= favorites),
    CHECK (learning >= completed),
    CHECK (views_uv >= 0 AND favorites >= 0 AND learning >= 0 AND completed >= 0),
    CHECK (examined >= 0 AND certified >= 0 AND orders >= 0 AND paid >= 0 AND unlock >= 0),
    CHECK (rating_count >= 0)
);

-- 平台业务参数：只放业务参数，绝不放密钥（密钥只走 shared/.env）。
CREATE TABLE IF NOT EXISTS system_config (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    value_type TEXT NOT NULL,
    min_value NUMERIC,
    max_value NUMERIC,
    description TEXT NOT NULL,
    updated_by INTEGER REFERENCES staff(id),
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (value_type IN ('int','bool','string','json'))
);

INSERT INTO system_config(key, value, value_type, min_value, max_value, description, updated_at)
VALUES ('stats.display_threshold', '10', 'int', 3, NULL, '课程统计人数低于此值时显示“少于 N 人”', CURRENT_TIMESTAMP)
ON CONFLICT (key) DO NOTHING;
INSERT INTO system_config(key, value, value_type, min_value, max_value, description, updated_at)
VALUES ('comment.display_threshold', '10', 'int', 3, NULL, '评价数低于此值时不展示均分', CURRENT_TIMESTAMP)
ON CONFLICT (key) DO NOTHING;
INSERT INTO system_config(key, value, value_type, min_value, max_value, description, updated_at)
VALUES ('offline.valid_days', '30', 'int', 1, 365, '离线下载授权有效期（天）', CURRENT_TIMESTAMP)
ON CONFLICT (key) DO NOTHING;
