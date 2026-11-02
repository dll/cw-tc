-- 注销审核（PRD §7.10.10（五）/ §7.1 A-4）
-- 学员提交注销申请 → 运营管理员审核 → 通过才执行匿名化；审核前必须校验进行中订单 / 退款。
CREATE TABLE IF NOT EXISTS deletion_request (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    student_id BIGINT NOT NULL REFERENCES student(id),
    reason TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending',
    handled_by BIGINT REFERENCES staff(id),
    handled_at TIMESTAMPTZ,
    result_note TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (status IN ('pending','approved','rejected')),
    CHECK (status = 'pending' OR result_note IS NOT NULL)
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_deletion_request_pending ON deletion_request(student_id) WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_deletion_request_status ON deletion_request(status, created_at);