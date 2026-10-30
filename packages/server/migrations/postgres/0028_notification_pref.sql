-- 消息偏好（PRD §7.13 L-5）：广播可退订，系统 / 事务通知不可退订。
CREATE TABLE IF NOT EXISTS notification_pref (
    student_id BIGINT PRIMARY KEY REFERENCES student(id) ON DELETE CASCADE,
    broadcast_opt_out BOOLEAN NOT NULL DEFAULT false,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
