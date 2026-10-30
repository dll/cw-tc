-- 消息偏好（PRD §7.13 L-5）：广播可退订，系统 / 事务通知不可退订。
CREATE TABLE IF NOT EXISTS notification_pref (
    student_id INTEGER PRIMARY KEY REFERENCES student(id) ON DELETE CASCADE,
    broadcast_opt_out INTEGER NOT NULL DEFAULT 0 CHECK (broadcast_opt_out IN (0,1)),
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
