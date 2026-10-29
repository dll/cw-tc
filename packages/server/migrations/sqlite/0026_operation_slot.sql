-- 运营位（PRD §7.10.12）：首页 banner / 首页推荐 / 广场置顶 / 专题。
CREATE TABLE IF NOT EXISTS operation_slot (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    position TEXT NOT NULL,
    course_code TEXT REFERENCES course(course_code),
    title TEXT NOT NULL DEFAULT '',
    image_url TEXT,
    link_url TEXT,
    starts_at TEXT NOT NULL,
    ends_at TEXT NOT NULL,
    weight INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'draft',
    created_by INTEGER REFERENCES staff(id),
    CHECK (position IN ('home_banner','home_recommend','plaza_top','topic')),
    CHECK (status IN ('draft','active','expired','removed')),
    CHECK (starts_at < ends_at)
);
CREATE INDEX IF NOT EXISTS idx_operation_slot_active ON operation_slot(position, status, weight DESC, starts_at);
