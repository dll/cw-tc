-- 离线下载授权（PRD §7.4 B-11 · 数据字典 §5.1.2）
-- 粒度到讲：仅已解锁课程可签发；离线有效期默认 30 天（system_config.offline.valid_days）。
CREATE TABLE IF NOT EXISTS download_grant (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    student_id BIGINT NOT NULL REFERENCES student(id),
    course_id BIGINT NOT NULL REFERENCES course(id),
    lecture_no INTEGER NOT NULL,
    granted_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    status TEXT NOT NULL DEFAULT 'valid',
    revoked_reason TEXT,
    CHECK (status IN ('valid','expired','revoked')),
    CHECK (expires_at > granted_at)
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_download_grant_lecture ON download_grant(student_id, course_id, lecture_no);
CREATE INDEX IF NOT EXISTS idx_download_grant_student ON download_grant(student_id, status);
-- @note 退款（normal）时同事务置 status='revoked' / revoked_reason='order_refunded'（commerce.Refund）。
-- @note reward 退款保留权益与证书，因此**不撤销**离线授权（§7.3 G-7）。
